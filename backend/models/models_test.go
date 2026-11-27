package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCategoryValid(t *testing.T) {
	for _, c := range Categories {
		if !c.Valid() {
			t.Errorf("%q should be valid", c)
		}
	}
	for _, c := range []EventCategory{"", "music", "Cinema"} {
		if c.Valid() {
			t.Errorf("%q should be invalid", c)
		}
	}
	if len(Categories) != 7 {
		t.Fatalf("categories = %v", Categories)
	}
}

func TestDefaultNotificationPreferencesJSON(t *testing.T) {
	b, err := json.Marshal(DefaultNotificationPreferences())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"savedEventReminders":true,"categoryNews":false,"categories":[],"newsletter":false,"publisherActivity":true}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
}

func TestApplySessions(t *testing.T) {
	e := &Event{Sessions: []EventSession{
		{Date: "2026-12-02", StartTime: "18:00", EndTime: "20:00"},
		{Date: "2026-12-01", StartTime: "22:00", EndTime: "02:00"},
		{Date: "2026-12-01", StartTime: "10:00", EndTime: "12:00"},
	}}
	e.ApplySessions()
	if e.Date != "2026-12-01" || e.StartTime != "10:00" || e.EndTime != "12:00" || e.EndDate != "2026-12-02" {
		t.Fatalf("derived %+v", e)
	}
	if e.Sessions[1].StartTime != "22:00" || e.Sessions[2].Date != "2026-12-02" {
		t.Fatalf("not sorted: %+v", e.Sessions)
	}

	empty := &Event{Date: "keep"}
	empty.ApplySessions()
	if empty.Sessions == nil || len(empty.Sessions) != 0 || empty.Date != "keep" {
		t.Fatalf("empty: %+v", empty)
	}
}

func TestApplyImages(t *testing.T) {
	e := &Event{StoredImageURL: "https://scraped/img.jpg"}
	e.ApplyImages()
	if e.Images == nil || e.ImageURL != "https://scraped/img.jpg" || e.Cover() != nil {
		t.Fatalf("no images: %+v", e)
	}
	e.Images = []EventImage{{ID: "g", Kind: ImageKindGallery, URL: "g.jpg"}, {ID: "c", Kind: ImageKindCover, URL: "c.jpg"}}
	e.ApplyImages()
	if e.ImageURL != "c.jpg" || e.Cover().ID != "c" {
		t.Fatalf("cover: %+v", e)
	}
}

func TestEventJSONShape(t *testing.T) {
	w := 10
	b, err := json.Marshal(Event{ID: "e", Sessions: []EventSession{{Date: "2026-12-01", StartTime: "10:00", EndTime: "11:00"}},
		Images:         []EventImage{{ID: "i", URL: "u", Kind: ImageKindCover, Width: &w, ObjectKey: "secret", EventID: "e"}},
		StoredImageURL: "hidden"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"11:00"}]`,
		`"images":[{"id":"i","url":"u","kind":"cover","position":0,"width":10}]`, `"endDate":""`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, "secret") || strings.Contains(s, "hidden") {
		t.Errorf("internal fields leaked: %s", s)
	}
}
