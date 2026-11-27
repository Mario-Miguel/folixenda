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
		e                  models.EventSession
		wantStart, wantEnd string
		wantErr            bool
	}{
		{"ok", models.EventSession{Date: "2026-11-27", StartTime: "20:30", EndTime: "23:00"},
			"2026-11-27 20:30", "2026-11-27 23:00", false},
		{"midnight", models.EventSession{Date: "2026-01-01", StartTime: "00:00", EndTime: "00:00"},
			"2026-01-01 00:00", "2026-01-01 00:00", false},
		{"overnight ends next day", models.EventSession{Date: "2026-11-27", StartTime: "22:00", EndTime: "02:00"},
			"2026-11-27 22:00", "2026-11-28 02:00", false},
		{"overnight across month and year", models.EventSession{Date: "2026-12-31", StartTime: "23:30", EndTime: "00:15"},
			"2026-12-31 23:30", "2027-01-01 00:15", false},
		{"one minute before start is next day", models.EventSession{Date: "2026-11-27", StartTime: "20:00", EndTime: "19:59"},
			"2026-11-27 20:00", "2026-11-28 19:59", false},
		{"equal times same instant", models.EventSession{Date: "2026-11-27", StartTime: "20:00", EndTime: "20:00"},
			"2026-11-27 20:00", "2026-11-27 20:00", false},
		{"bad date", models.EventSession{Date: "27/11/2026", StartTime: "20:30", EndTime: "23:00"}, "", "", true},
		{"bad start", models.EventSession{Date: "2026-11-27", StartTime: "8pm", EndTime: "23:00"}, "", "", true},
		{"missing end", models.EventSession{Date: "2026-11-27", StartTime: "20:30"}, "", "", true},
		{"hour out of range", models.EventSession{Date: "2026-11-27", StartTime: "25:00", EndTime: "23:00"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := eventTimestamps(tt.e)
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

func TestBuildSessionRows(t *testing.T) {
	rows, err := buildSessionRows([]models.EventSession{
		{Date: "2026-12-02", StartTime: "22:00", EndTime: "01:00"},
		{Date: "2026-12-01", StartTime: "10:00", EndTime: "12:00"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rows.spanStart.Format(dateTimeLayout); got != "2026-12-01 10:00" {
		t.Errorf("span start %s", got)
	}
	if got := rows.spanEnd.Format(dateTimeLayout); got != "2026-12-03 01:00" {
		t.Errorf("span end %s", got)
	}
	if !reflect.DeepEqual(rows.starts, []string{"2026-12-02 22:00:00", "2026-12-01 10:00:00"}) ||
		!reflect.DeepEqual(rows.ends, []string{"2026-12-03 01:00:00", "2026-12-01 12:00:00"}) {
		t.Errorf("rows %+v", rows)
	}
	if _, err := buildSessionRows(nil); err == nil {
		t.Error("no sessions must fail")
	}
	if _, err := buildSessionRows([]models.EventSession{{Date: "2026-12-01", StartTime: "10:00", EndTime: "11:00"},
		{Date: "bad"}}); err == nil || !strings.Contains(err.Error(), "session 2") {
		t.Errorf("err = %v", err)
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

func TestQualifiedColumns(t *testing.T) {
	if got := qualifiedColumns("", []string{"a", "b"}); got != "a, b" {
		t.Fatalf("got %q", got)
	}
	if got := qualifiedColumns("e.", []string{"a", "b"}); got != "e.a, e.b" {
		t.Fatalf("got %q", got)
	}
	if got := qualifiedColumns("e.", nil); got != "" {
		t.Fatalf("got %q", got)
	}
	if !strings.HasPrefix(eventColumnsE, "e.id, ") || !strings.Contains(eventColumnsE, "e.publisher_id,") ||
		!strings.Contains(eventColumnsE, "FROM event_sessions s WHERE s.event_id = e.id") ||
		!strings.HasSuffix(eventColumnsE, "FROM event_images i WHERE i.event_id = e.id)") {
		t.Fatalf("unexpected select list: %q", eventColumnsE)
	}
}

func TestNullIfEmpty(t *testing.T) {
	if nullIfEmpty("") != nil {
		t.Fatal(`"" must be NULL`)
	}
	if nullIfEmpty("x") != "x" {
		t.Fatal("non-empty must pass through")
	}
}

func TestScanPublisherProfile(t *testing.T) {
	created := time.Date(2026, 11, 1, 10, 0, 0, 0, time.FixedZone("CET", 3600))
	p, err := scanPublisherProfile(rowScanner{vals: []any{"u1", "Acme", nil, nil, nil, created, created.Add(time.Hour)}})
	if err != nil {
		t.Fatal(err)
	}
	want := models.PublisherProfile{UserID: "u1", OrgName: "Acme",
		CreatedAt: "2026-11-01T09:00:00Z", UpdatedAt: "2026-11-01T10:00:00Z"}
	if *p != want {
		t.Fatalf("got %+v", *p)
	}
	if _, err := scanPublisherProfile(rowScanner{err: errors.New("x")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestScanNotificationPreferences(t *testing.T) {
	p, err := scanNotificationPreferences(rowScanner{vals: []any{true, false, []byte(`{}`), false, true}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Categories == nil || len(p.Categories) != 0 || !p.SavedEventReminders || !p.PublisherActivity {
		t.Fatalf("got %+v", p)
	}
	p, err = scanNotificationPreferences(rowScanner{vals: []any{false, true, []byte(`{Music,Art}`), true, false}})
	if err != nil || !reflect.DeepEqual(p.Categories, []models.EventCategory{"Music", "Art"}) || !p.Newsletter {
		t.Fatalf("got %+v err %v", p, err)
	}
	if _, err := scanNotificationPreferences(rowScanner{err: errors.New("x")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildListEventsQuery(t *testing.T) {
	const lateral = "JOIN LATERAL (SELECT min(s.start_time) AS first_start FROM event_sessions s\n\t\t\tWHERE s.event_id = e.id AND "
	tests := []struct {
		name         string
		f            EventFilter
		wantSessions string // conditions inside the lateral join; "" = no join
		wantWhere    string // "" means no WHERE clause
		wantArgs     []any
	}{
		{"no filter", EventFilter{}, "", "", nil},
		{"date: sessions starting that day", EventFilter{Date: "2026-11-27"},
			"s.start_time >= $1::date AND s.start_time < $1::date + 1", "", []any{"2026-11-27"}},
		{"category", EventFilter{Category: "Music"}, "", "e.category = $1", []any{"Music"}},
		{"range", EventFilter{From: "2026-11-01", To: "2026-11-30"},
			"s.start_time >= $1::date AND s.start_time < $2::date + 1", "", []any{"2026-11-01", "2026-11-30"}},
		{"from + category", EventFilter{From: "2026-11-01", Category: "Art"},
			"s.start_time >= $1::date", "e.category = $2", []any{"2026-11-01", "Art"}},
		{"all", EventFilter{Date: "2026-11-05", From: "2026-11-01", To: "2026-11-30", Category: "Food"},
			"s.start_time >= $1::date AND s.start_time < $1::date + 1 AND s.start_time >= $2::date AND s.start_time < $3::date + 1",
			"e.category = $4", []any{"2026-11-05", "2026-11-01", "2026-11-30", "Food"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, args := buildListEventsQuery(tt.f)
			if !strings.HasPrefix(q, "SELECT "+eventColumnsE+"\n\t\tFROM events e") {
				t.Fatalf("unexpected query %q", q)
			}
			if tt.wantSessions == "" {
				if strings.Contains(q, "LATERAL") || !strings.HasSuffix(q, "ORDER BY e.start_time, e.id") {
					t.Fatalf("query %q", q)
				}
			} else {
				if !strings.Contains(q, lateral+tt.wantSessions+") m ON m.first_start IS NOT NULL") {
					t.Fatalf("query %q\nmissing sessions %q", q, tt.wantSessions)
				}
				if !strings.HasSuffix(q, "ORDER BY m.first_start, e.id") {
					t.Fatalf("must order by first matching session: %q", q)
				}
			}
			if tt.wantWhere == "" {
				if strings.Contains(q, "\n\t\tWHERE") {
					t.Fatalf("unexpected WHERE in %q", q)
				}
			} else if !strings.Contains(q, "\n\t\tWHERE "+tt.wantWhere+"\n") {
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

const (
	twoSessionsJSON = `[{"date":"2026-11-27","startTime":"20:30","endTime":"23:15"},{"date":"2026-11-28","startTime":"22:00","endTime":"02:00"}]`
	imagesJSON      = `[{"id":"c1","key":"events/e1/c1.jpg","kind":"cover","position":0,"width":800,"height":600},` +
		`{"id":"g1","key":"events/e1/g1.png","kind":"gallery","position":0,"width":null,"height":null}]`
)

func eventRow(lat, lon any) []any {
	start := time.Date(2026, 11, 27, 20, 30, 0, 0, time.UTC)
	end := time.Date(2026, 11, 29, 2, 0, 0, 0, time.UTC)
	return []any{"e1", "Jazz", "desc", "Music", start, end, 12.5, "Venue", "Addr",
		"img", "Artist", []byte(`{vip,drinks}`), true, lat, lon, "pub-1", []byte(twoSessionsJSON), []byte(`[]`)}
}

func cdn(key string) string { return "https://cdn/" + key }

func TestScanEvent(t *testing.T) {
	lat, lon := 43.3, -5.8

	e, err := scanEvent(rowScanner{vals: eventRow(lat, lon)}, cdn)
	if err != nil {
		t.Fatal(err)
	}
	want := models.Event{ID: "e1", Title: "Jazz", Description: "desc", Category: "Music",
		Date: "2026-11-27", StartTime: "20:30", EndTime: "23:15", EndDate: "2026-11-28",
		Sessions: []models.EventSession{{Date: "2026-11-27", StartTime: "20:30", EndTime: "23:15"},
			{Date: "2026-11-28", StartTime: "22:00", EndTime: "02:00"}},
		Venue: "Venue", Address: "Addr", Price: 12.5, ImageURL: "img", StoredImageURL: "img", Images: []models.EventImage{},
		ArtistName: "Artist", Perks: []string{"vip", "drinks"}, IsSaved: true, PublisherID: "pub-1"}
	got := *e
	if got.Lat == nil || *got.Lat != lat || got.Lon == nil || *got.Lon != lon {
		t.Fatalf("lat/lon = %v/%v", got.Lat, got.Lon)
	}
	got.Lat, got.Lon = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}

	t.Run("images: cover URL wins over image_url", func(t *testing.T) {
		row := eventRow(nil, nil)
		row[17] = []byte(imagesJSON)
		e, err := scanEvent(rowScanner{vals: row}, cdn)
		if err != nil {
			t.Fatal(err)
		}
		w, h := 800, 600
		wantImgs := []models.EventImage{
			{ID: "c1", URL: "https://cdn/events/e1/c1.jpg", Kind: "cover", Width: &w, Height: &h, EventID: "e1", ObjectKey: "events/e1/c1.jpg"},
			{ID: "g1", URL: "https://cdn/events/e1/g1.png", Kind: "gallery", EventID: "e1", ObjectKey: "events/e1/g1.png"},
		}
		if !reflect.DeepEqual(e.Images, wantImgs) || e.ImageURL != "https://cdn/events/e1/c1.jpg" || e.StoredImageURL != "img" {
			t.Fatalf("images %+v imageUrl %q", e.Images, e.ImageURL)
		}
		if e, _ := scanEvent(rowScanner{vals: row}, nil); e.Images[0].URL != "events/e1/c1.jpg" {
			t.Fatalf("nil URLFunc must leave the key: %q", e.Images[0].URL)
		}
	})
	t.Run("no session rows falls back to the event span", func(t *testing.T) {
		row := eventRow(nil, nil)
		row[16] = []byte(`[]`)
		e, err := scanEvent(rowScanner{vals: row}, cdn)
		if err != nil {
			t.Fatal(err)
		}
		if len(e.Sessions) != 1 || e.Sessions[0] != (models.EventSession{Date: "2026-11-27", StartTime: "20:30", EndTime: "02:00"}) ||
			e.EndDate != "2026-11-27" {
			t.Fatalf("got %+v", e)
		}
		row[5] = nil
		e, _ = scanEvent(rowScanner{vals: row}, cdn)
		if e.Sessions[0].EndTime != "20:30" {
			t.Fatalf("null end: %+v", e.Sessions)
		}
	})
	t.Run("nulls", func(t *testing.T) {
		row := []any{"e2", "T", nil, "Art", nil, nil, 0.0, nil, nil, nil, nil, nil, false, nil, nil, nil, nil, nil}
		e, err := scanEvent(rowScanner{vals: row}, cdn)
		if err != nil {
			t.Fatal(err)
		}
		if e.Description != "" || e.Date != "" || e.StartTime != "" || e.EndTime != "" || e.EndDate != "" ||
			e.Venue != "" || e.Perks != nil || e.Lat != nil || e.Lon != nil || e.PublisherID != "" ||
			e.Sessions == nil || len(e.Sessions) != 0 || e.Images == nil || len(e.Images) != 0 || e.ImageURL != "" {
			t.Fatalf("got %+v", e)
		}
	})
	t.Run("bad sessions json", func(t *testing.T) {
		row := eventRow(nil, nil)
		row[16] = []byte(`{`)
		if _, err := scanEvent(rowScanner{vals: row}, cdn); err == nil || !strings.Contains(err.Error(), "sessions") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad images json", func(t *testing.T) {
		row := eventRow(nil, nil)
		row[17] = []byte(`{`)
		if _, err := scanEvent(rowScanner{vals: row}, cdn); err == nil || !strings.Contains(err.Error(), "images") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("only lat set means no coordinates", func(t *testing.T) {
		e, err := scanEvent(rowScanner{vals: eventRow(lat, nil)}, cdn)
		if err != nil || e.Lat != nil || e.Lon != nil {
			t.Fatalf("e=%+v err=%v", e, err)
		}
	})
	t.Run("scan error", func(t *testing.T) {
		if _, err := scanEvent(rowScanner{err: errors.New("x")}, cdn); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestScanSavedEvent(t *testing.T) {
	saved := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	row := append(eventRow(nil, nil), "user-1", saved)
	userID, se, err := scanSavedEvent(rowScanner{vals: row}, cdn)
	if err != nil {
		t.Fatal(err)
	}
	if userID != "user-1" || se.ID != "e1" || se.CreatedAt != "2026-11-20T09:00:00Z" {
		t.Fatalf("userID=%s se=%+v", userID, se)
	}

	row = append(eventRow(nil, nil), "user-1", nil)
	if _, se, err := scanSavedEvent(rowScanner{vals: row}, cdn); err != nil || se.CreatedAt != "" {
		t.Fatalf("null created_at: se=%+v err=%v", se, err)
	}
	if _, _, err := scanSavedEvent(rowScanner{err: errors.New("x")}, cdn); err == nil {
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
