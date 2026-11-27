package store

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

func TestEventTimestamps(t *testing.T) {
	tests := []struct {
		name               string
		e                  models.Event
		wantStart, wantEnd string
		wantErr            bool
	}{
		{"ok", models.Event{Date: "2026-11-27", StartTime: "20:30", EndTime: "23:00"},
			"2026-11-27 20:30", "2026-11-27 23:00", false},
		{"midnight", models.Event{Date: "2026-01-01", StartTime: "00:00", EndTime: "00:00"},
			"2026-01-01 00:00", "2026-01-01 00:00", false},
		{"bad date", models.Event{Date: "27/11/2026", StartTime: "20:30", EndTime: "23:00"}, "", "", true},
		{"bad start", models.Event{Date: "2026-11-27", StartTime: "8pm", EndTime: "23:00"}, "", "", true},
		{"missing end", models.Event{Date: "2026-11-27", StartTime: "20:30"}, "", "", true},
		{"hour out of range", models.Event{Date: "2026-11-27", StartTime: "25:00", EndTime: "23:00"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := eventTimestamps(&tt.e)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got := start.Format(dateTimeLayout); got != tt.wantStart {
				t.Errorf("start = %s, want %s", got, tt.wantStart)
			}
			if got := end.Format(dateTimeLayout); got != tt.wantEnd {
				t.Errorf("end = %s, want %s", got, tt.wantEnd)
			}
		})
	}
}

func TestEventFilterValidate(t *testing.T) {
	tests := []struct {
		name    string
		f       EventFilter
		wantErr string
	}{
		{"empty", EventFilter{}, ""},
		{"date", EventFilter{Date: "2026-11-27"}, ""},
		{"range", EventFilter{From: "2026-11-01", To: "2026-11-30"}, ""},
		{"same day", EventFilter{From: "2026-11-01", To: "2026-11-01"}, ""},
		{"only from", EventFilter{From: "2026-11-01"}, ""},
		{"only to", EventFilter{To: "2026-11-01"}, ""},
		{"everything", EventFilter{Date: "2026-11-05", From: "2026-11-01", To: "2026-11-30", Category: "Music"}, ""},
		{"category is free text", EventFilter{Category: "anything goes"}, ""},
		{"bad date", EventFilter{Date: "2026-11-31"}, "invalid date"},
		{"bad from", EventFilter{From: "20261101"}, "invalid from"},
		{"bad to", EventFilter{To: "2026-1-1"}, "invalid to"},
		{"from after to", EventFilter{From: "2026-12-01", To: "2026-11-01"}, "after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuildListEventsQuery(t *testing.T) {
	tests := []struct {
		name      string
		f         EventFilter
		wantWhere string // "" means no WHERE clause
		wantArgs  []any
	}{
		{"no filter", EventFilter{}, "", nil},
		{"date", EventFilter{Date: "2026-11-27"}, "start_time::date = $1::date", []any{"2026-11-27"}},
		{"category", EventFilter{Category: "Music"}, "category = $1", []any{"Music"}},
		{"range", EventFilter{From: "2026-11-01", To: "2026-11-30"},
			"start_time::date >= $1::date AND start_time::date <= $2::date", []any{"2026-11-01", "2026-11-30"}},
		{"from + category", EventFilter{From: "2026-11-01", Category: "Art"},
			"start_time::date >= $1::date AND category = $2", []any{"2026-11-01", "Art"}},
		{"all", EventFilter{Date: "2026-11-05", From: "2026-11-01", To: "2026-11-30", Category: "Food"},
			"start_time::date = $1::date AND start_time::date >= $2::date AND start_time::date <= $3::date AND category = $4",
			[]any{"2026-11-05", "2026-11-01", "2026-11-30", "Food"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, args := buildListEventsQuery(tt.f)
			if !strings.HasPrefix(q, "SELECT "+eventColumns) || !strings.Contains(q, "FROM events") {
				t.Fatalf("unexpected query %q", q)
			}
			if !strings.HasSuffix(q, "ORDER BY start_time, id") {
				t.Fatalf("query must be ordered: %q", q)
			}
			if tt.wantWhere == "" {
				if strings.Contains(q, "WHERE") {
					t.Fatalf("unexpected WHERE in %q", q)
				}
			} else if !strings.Contains(q, "WHERE "+tt.wantWhere+"\n") {
				t.Fatalf("query %q\nmissing WHERE %q", q, tt.wantWhere)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestTranslateError(t *testing.T) {
	plain := errors.New("boom")
	tests := []struct {
		name   string
		err    error
		wantIs error
	}{
		{"nil", nil, nil},
		{"plain error unchanged", plain, plain},
		{"unique violation", &pq.Error{Code: "23505"}, ErrConflict},
		{"fk violation", &pq.Error{Code: "23503"}, ErrNotFound},
		{"invalid uuid", &pq.Error{Code: "22P02"}, ErrNotFound},
		{"other pq error unchanged", &pq.Error{Code: "42P01"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Fatalf("got %v", got)
				}
				return
			}
			if tt.wantIs == nil {
				if got != tt.err {
					t.Fatalf("got %v, want unchanged %v", got, tt.err)
				}
				return
			}
			if !errors.Is(got, tt.wantIs) {
				t.Fatalf("got %v, want errors.Is %v", got, tt.wantIs)
			}
			var pqErr *pq.Error
			if _, isPQ := tt.err.(*pq.Error); isPQ && !errors.As(got, &pqErr) {
				t.Fatalf("original pq error lost: %v", got)
			}
		})
	}
}

// rowScanner is a fake scanner that assigns vals to the destinations the way
// database/sql would (via sql.Scanner or direct assignment).
type rowScanner struct {
	vals []any
	err  error
}

func (r rowScanner) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return errors.New("column count mismatch")
	}
	for i, d := range dest {
		v := r.vals[i]
		if s, ok := d.(sql.Scanner); ok {
			if err := s.Scan(v); err != nil {
				return err
			}
			continue
		}
		rv := reflect.ValueOf(d).Elem()
		if v == nil {
			rv.Set(reflect.Zero(rv.Type()))
			continue
		}
		rv.Set(reflect.ValueOf(v).Convert(rv.Type()))
	}
	return nil
}

func eventRow(lat, lon any) []any {
	start := time.Date(2026, 11, 27, 20, 30, 0, 0, time.UTC)
	end := time.Date(2026, 11, 27, 23, 15, 0, 0, time.UTC)
	return []any{"e1", "Jazz", "desc", "Music", start, end, 12.5, "Venue", "Addr",
		"img", "Artist", []byte(`{vip,drinks}`), true, lat, lon}
}

func TestScanEvent(t *testing.T) {
	lat, lon := 43.3, -5.8

	e, err := scanEvent(rowScanner{vals: eventRow(lat, lon)})
	if err != nil {
		t.Fatal(err)
	}
	want := models.Event{ID: "e1", Title: "Jazz", Description: "desc", Category: "Music",
		Date: "2026-11-27", StartTime: "20:30", EndTime: "23:15", Venue: "Venue", Address: "Addr",
		Price: 12.5, ImageURL: "img", ArtistName: "Artist", Perks: []string{"vip", "drinks"}, IsSaved: true}
	got := *e
	if got.Lat == nil || *got.Lat != lat || got.Lon == nil || *got.Lon != lon {
		t.Fatalf("lat/lon = %v/%v", got.Lat, got.Lon)
	}
	got.Lat, got.Lon = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}

	t.Run("nulls", func(t *testing.T) {
		row := []any{"e2", "T", nil, "Art", nil, nil, 0.0, nil, nil, nil, nil, nil, false, nil, nil}
		e, err := scanEvent(rowScanner{vals: row})
		if err != nil {
			t.Fatal(err)
		}
		if e.Description != "" || e.Date != "" || e.StartTime != "" || e.EndTime != "" ||
			e.Venue != "" || e.Perks != nil || e.Lat != nil || e.Lon != nil {
			t.Fatalf("got %+v", e)
		}
	})
	t.Run("only lat set means no coordinates", func(t *testing.T) {
		e, err := scanEvent(rowScanner{vals: eventRow(lat, nil)})
		if err != nil || e.Lat != nil || e.Lon != nil {
			t.Fatalf("e=%+v err=%v", e, err)
		}
	})
	t.Run("scan error", func(t *testing.T) {
		if _, err := scanEvent(rowScanner{err: errors.New("x")}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestScanSavedEvent(t *testing.T) {
	saved := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	row := append(eventRow(nil, nil), "user-1", saved)
	userID, se, err := scanSavedEvent(rowScanner{vals: row})
	if err != nil {
		t.Fatal(err)
	}
	if userID != "user-1" || se.ID != "e1" || se.CreatedAt != "2026-11-20T09:00:00Z" {
		t.Fatalf("userID=%s se=%+v", userID, se)
	}

	row = append(eventRow(nil, nil), "user-1", nil)
	if _, se, err := scanSavedEvent(rowScanner{vals: row}); err != nil || se.CreatedAt != "" {
		t.Fatalf("null created_at: se=%+v err=%v", se, err)
	}
	if _, _, err := scanSavedEvent(rowScanner{err: errors.New("x")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestScanUser(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		u, err := scanUser(rowScanner{vals: []any{"u1", "a@b.c", "Ana", "admin", "pro", "card",
			[]byte(`{43.3,-5.8}`), []byte(`{Music,Art}`)}})
		if err != nil {
			t.Fatal(err)
		}
		want := models.User{ID: "u1", Email: "a@b.c", Name: "Ana", Role: "admin", SubscriptionType: "pro",
			PaymentMethod: "card", Location: []float64{43.3, -5.8}, EventPreferences: []string{"Music", "Art"}}
		if !reflect.DeepEqual(*u, want) {
			t.Fatalf("got %+v", *u)
		}
	})
	t.Run("nullable columns (as written by Upsert)", func(t *testing.T) {
		u, err := scanUser(rowScanner{vals: []any{"u1", "a@b.c", nil, "consumer", nil, nil, nil, nil}})
		if err != nil {
			t.Fatal(err)
		}
		if u.Name != "" || u.SubscriptionType != "" || u.PaymentMethod != "" || u.Location != nil || u.EventPreferences != nil {
			t.Fatalf("got %+v", *u)
		}
	})
	t.Run("scan error", func(t *testing.T) {
		if _, err := scanUser(rowScanner{err: errors.New("x")}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestExtraScanner(t *testing.T) {
	var a, b, c string
	x := extraScanner{s: rowScanner{vals: []any{"1", "2", "3"}}, extra: []any{&c}}
	if err := x.Scan(&a, &b); err != nil {
		t.Fatal(err)
	}
	if a != "1" || b != "2" || c != "3" {
		t.Fatalf("a=%s b=%s c=%s", a, b, c)
	}
}
