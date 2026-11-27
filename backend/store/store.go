package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02 15:04"
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

func eventTimestamps(e *models.Event) (startTS time.Time, endTS time.Time, err error) {
	startTS, err = time.Parse(dateTimeLayout, e.Date+" "+e.StartTime)
	if err != nil {
		return
	}
	endTS, err = time.Parse(dateTimeLayout, e.Date+" "+e.EndTime)
	return
}

// EventFilter narrows the events returned by EventStore.List. Empty fields
// don't filter. Dates are YYYY-MM-DD and apply to the event start date; From
// and To are both inclusive.
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

const eventColumns = `id, title, description, category, start_time, end_time, price, venue, address,
		       image_url, artist_name, perks, is_saved, lat, lon`

// buildListEventsQuery returns the SELECT for EventStore.List and its args.
// Results are ordered by start time, then id, for stable output.
func buildListEventsQuery(f EventFilter) (string, []any) {
	var where []string
	var args []any
	add := func(cond string, arg any) {
		args = append(args, arg)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Date != "" {
		add("start_time::date = $%d::date", f.Date)
	}
	if f.From != "" {
		add("start_time::date >= $%d::date", f.From)
	}
	if f.To != "" {
		add("start_time::date <= $%d::date", f.To)
	}
	if f.Category != "" {
		add("category = $%d", f.Category)
	}

	q := "SELECT " + eventColumns + "\n\t\tFROM events"
	if len(where) > 0 {
		q += "\n\t\tWHERE " + strings.Join(where, " AND ")
	}
	q += "\n\t\tORDER BY start_time, id"
	return q, args
}
