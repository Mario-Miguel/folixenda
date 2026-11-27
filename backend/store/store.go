package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

const (
	dateLayout         = "2006-01-02"
	dateTimeLayout     = "2006-01-02 15:04"
	sqlTimestampLayout = "2006-01-02 15:04:05"
)

type scanner interface {
	Scan(dest ...any) error
}

// extraScanner appends extra destinations after the ones passed to Scan,
// letting scanEvent be reused for rows that carry additional columns.
type extraScanner struct {
	s     scanner
	extra []any
}

func (x extraScanner) Scan(dest ...any) error {
	return x.s.Scan(append(dest, x.extra...)...)
}

// eventTimestamps builds a session's start/end from its date and clock
// times. An end clock time earlier than the start (e.g. 22:00–02:00) means the
// session ends the next day; equal times are the same instant.
func eventTimestamps(s models.EventSession) (startTS time.Time, endTS time.Time, err error) {
	startTS, err = time.Parse(dateTimeLayout, s.Date+" "+s.StartTime)
	if err != nil {
		return
	}
	endTS, err = time.Parse(dateTimeLayout, s.Date+" "+s.EndTime)
	if err == nil && endTS.Before(startTS) {
		endTS = endTS.AddDate(0, 0, 1)
	}
	return
}

// sessionRows is an event's sessions ready to be written: the overall span
// (first start, latest end) for events.start_time/end_time, and the
// timestamps of every session as text arrays for unnest.
type sessionRows struct {
	spanStart, spanEnd time.Time
	starts, ends       []string
}

// buildSessionRows converts sessions (at least one) into sessionRows.
func buildSessionRows(sessions []models.EventSession) (sessionRows, error) {
	var out sessionRows
	if len(sessions) == 0 {
		return out, errors.New("an event needs at least one session")
	}
	for i, s := range sessions {
		start, end, err := eventTimestamps(s)
		if err != nil {
			return out, fmt.Errorf("session %d: %w", i+1, err)
		}
		if i == 0 || start.Before(out.spanStart) {
			out.spanStart = start
		}
		if i == 0 || end.After(out.spanEnd) {
			out.spanEnd = end
		}
		out.starts = append(out.starts, start.Format(sqlTimestampLayout))
		out.ends = append(out.ends, end.Format(sqlTimestampLayout))
	}
	return out, nil
}

// EventFilter narrows the events returned by EventStore.List. Empty fields
// don't filter. Dates are YYYY-MM-DD and match an event when any of its
// sessions starts on that day (Date) or within [From, To] (both inclusive).
type EventFilter struct {
	Date     string
	From     string
	To       string
	Category string
}

// Validate checks that every non-empty date is YYYY-MM-DD and that From <= To.
func (f EventFilter) Validate() error {
	for _, d := range []struct{ name, value string }{
		{"date", f.Date}, {"from", f.From}, {"to", f.To},
	} {
		if d.value == "" {
			continue
		}
		if _, err := time.Parse(dateLayout, d.value); err != nil {
			return fmt.Errorf("invalid %s %q: expected YYYY-MM-DD", d.name, d.value)
		}
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		return fmt.Errorf("from (%s) is after to (%s)", f.From, f.To)
	}
	return nil
}

// eventColumnNames are the events columns read by scanEvent, in scan order.
// scanEvent also reads two JSON columns after them: sessionsColumn and
// imagesColumn.
var eventColumnNames = []string{"id", "title", "description", "category", "start_time", "end_time",
	"price", "venue", "address", "image_url", "artist_name", "perks", "is_saved", "lat", "lon",
	"publisher_id"}

// qualifiedColumns returns names joined by ", ", each prefixed with prefix
// (e.g. "e." for queries that join other tables).
func qualifiedColumns(prefix string, names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = prefix + n
	}
	return strings.Join(out, ", ")
}

const (
	// sessionsColumn is a JSON array of the event's sessions, sorted, in the
	// models.EventSession shape. Requires events aliased as "e".
	sessionsColumn = `(SELECT COALESCE(json_agg(json_build_object(
			'date', to_char(s.start_time, 'YYYY-MM-DD'),
			'startTime', to_char(s.start_time, 'HH24:MI'),
			'endTime', to_char(s.end_time, 'HH24:MI')) ORDER BY s.start_time), '[]'::json)
		FROM event_sessions s WHERE s.event_id = e.id)`
	// imagesColumn is a JSON array of the event's images in imageJSON shape:
	// cover first, then the gallery by position. Requires events aliased as "e".
	imagesColumn = `(SELECT COALESCE(json_agg(json_build_object(
			'id', i.id, 'key', i.object_key, 'kind', i.kind, 'position', i.position,
			'width', i.width, 'height', i.height) ORDER BY i.kind = 'gallery', i.position, i.created_at), '[]'::json)
		FROM event_images i WHERE i.event_id = e.id)`
)

// eventColumnsE is the SELECT list for scanEvent, for queries aliasing events
// as "e". Sessions and images come from correlated subqueries (one query, no
// N+1).
var eventColumnsE = qualifiedColumns("e.", eventColumnNames) + ",\n\t\t" + sessionsColumn + ",\n\t\t" + imagesColumn

// nullIfEmpty maps "" to a SQL NULL, for nullable text/uuid columns.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// buildListEventsQuery returns the SELECT for EventStore.List and its args.
//
// Without date filters, events are ordered by their start, then id. With
// date filters, an event matches when any of its sessions starts in range,
// and events are ordered by their first matching session, then id.
func buildListEventsQuery(f EventFilter) (string, []any) {
	var sessionConds, where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	// Ranges on s.start_time (not s.start_time::date) so the index is usable.
	if f.Date != "" {
		p := arg(f.Date)
		sessionConds = append(sessionConds, "s.start_time >= "+p+"::date", "s.start_time < "+p+"::date + 1")
	}
	if f.From != "" {
		sessionConds = append(sessionConds, "s.start_time >= "+arg(f.From)+"::date")
	}
	if f.To != "" {
		sessionConds = append(sessionConds, "s.start_time < "+arg(f.To)+"::date + 1")
	}
	if f.Category != "" {
		where = append(where, "e.category = "+arg(f.Category))
	}

	q := "SELECT " + eventColumnsE + "\n\t\tFROM events e"
	order := "e.start_time, e.id"
	if len(sessionConds) > 0 {
		q += "\n\t\tJOIN LATERAL (SELECT min(s.start_time) AS first_start FROM event_sessions s" +
			"\n\t\t\tWHERE s.event_id = e.id AND " + strings.Join(sessionConds, " AND ") +
			") m ON m.first_start IS NOT NULL"
		order = "m.first_start, e.id"
	}
	if len(where) > 0 {
		q += "\n\t\tWHERE " + strings.Join(where, " AND ")
	}
	q += "\n\t\tORDER BY " + order
	return q, args
}
