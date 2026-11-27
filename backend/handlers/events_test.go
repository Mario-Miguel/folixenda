package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func sampleEvents() []*models.Event {
	return []*models.Event{
		{ID: "e1", Title: "Jazz", Category: models.CategoryMusic, Date: "2026-11-01", StartTime: "20:00", EndTime: "22:00"},
		{ID: "e2", Title: "Hamlet", Category: models.CategoryTheater, Date: "2026-11-05", StartTime: "19:00", EndTime: "21:00"},
		{ID: "e3", Title: "Rock", Category: models.CategoryMusic, Date: "2026-11-10", StartTime: "21:00", EndTime: "23:30"},
	}
}

func eventsMux(s store.EventStore) *http.ServeMux {
	mux := http.NewServeMux()
	NewEventsHandler(s).Register(mux, fakeProtect)
	return mux
}

func ids(events []*models.Event) string {
	var out []string
	for _, e := range events {
		out = append(out, e.ID)
	}
	return strings.Join(out, ",")
}

func TestEventsList(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		storeErr   error
		wantStatus int
		wantIDs    string
		wantFilter store.EventFilter
	}{
		{name: "all", query: "", wantStatus: 200, wantIDs: "e1,e2,e3"},
		{name: "by date", query: "?date=2026-11-05", wantStatus: 200, wantIDs: "e2",
			wantFilter: store.EventFilter{Date: "2026-11-05"}},
		{name: "by category", query: "?category=Music", wantStatus: 200, wantIDs: "e1,e3",
			wantFilter: store.EventFilter{Category: "Music"}},
		{name: "range inclusive", query: "?from=2026-11-01&to=2026-11-05", wantStatus: 200, wantIDs: "e1,e2",
			wantFilter: store.EventFilter{From: "2026-11-01", To: "2026-11-05"}},
		{name: "range + category", query: "?from=2026-11-01&to=2026-11-30&category=Music", wantStatus: 200,
			wantIDs: "e1,e3", wantFilter: store.EventFilter{From: "2026-11-01", To: "2026-11-30", Category: "Music"}},
		{name: "open-ended from", query: "?from=2026-11-06", wantStatus: 200, wantIDs: "e3",
			wantFilter: store.EventFilter{From: "2026-11-06"}},
		{name: "open-ended to", query: "?to=2026-11-01", wantStatus: 200, wantIDs: "e1",
			wantFilter: store.EventFilter{To: "2026-11-01"}},
		{name: "same day range", query: "?from=2026-11-10&to=2026-11-10", wantStatus: 200, wantIDs: "e3",
			wantFilter: store.EventFilter{From: "2026-11-10", To: "2026-11-10"}},
		{name: "no matches is empty list", query: "?date=2027-01-01", wantStatus: 200, wantIDs: "",
			wantFilter: store.EventFilter{Date: "2027-01-01"}},
		{name: "bad date", query: "?date=05/11/2026", wantStatus: 400},
		{name: "bad from", query: "?from=2026-13-01", wantStatus: 400},
		{name: "bad to", query: "?to=tomorrow", wantStatus: 400},
		{name: "from after to", query: "?from=2026-11-10&to=2026-11-01", wantStatus: 400},
		{name: "store error", query: "", storeErr: errDB, wantStatus: 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			s.err = tt.storeErr
			rec := do(t, eventsMux(s), "GET", "/api/events"+tt.query, "", anon)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 200 {
				if decode[map[string]string](t, rec)["error"] == "" {
					t.Fatal("expected an error message")
				}
				return
			}
			resp := decode[listResponse](t, rec)
			if resp.Events == nil {
				t.Fatal("events must be [] not null")
			}
			if got := ids(resp.Events); got != tt.wantIDs {
				t.Fatalf("ids = %q, want %q", got, tt.wantIDs)
			}
			if resp.Total != len(resp.Events) {
				t.Fatalf("total = %d, len = %d", resp.Total, len(resp.Events))
			}
			if s.lastFilter != tt.wantFilter {
				t.Fatalf("filter = %+v, want %+v", s.lastFilter, tt.wantFilter)
			}
		})
	}
}

func TestEventsGet(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		storeErr   error
		wantStatus int
	}{
		{"found", "e2", nil, 200},
		{"not found", "nope", nil, 404},
		{"store error", "e2", errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			s.err = tt.storeErr
			rec := do(t, eventsMux(s), "GET", "/api/events/"+tt.id, "", anon)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == 200 && decode[models.Event](t, rec).Title != "Hamlet" {
				t.Fatalf("body = %s", rec.Body)
			}
		})
	}
}

const validEventJSON = `{"title":"New","category":"Art","date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`

func TestEventsCreate(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		body       string
		storeErr   error
		wantStatus int
	}{
		{"unauthenticated", anon, validEventJSON, nil, 401},
		{"consumer forbidden", consumer, validEventJSON, nil, 403},
		{"no role forbidden", identity{"x", ""}, validEventJSON, nil, 403},
		{"publisher creates", publisher, validEventJSON, nil, 201},
		{"admin creates", admin, validEventJSON, nil, 201},
		{"bad json", admin, `{"title":`, nil, 400},
		{"missing title", admin, `{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`, nil, 400},
		{"bad date", admin, `{"title":"x","date":"01-12-2026","startTime":"10:00","endTime":"12:00"}`, nil, 400},
		{"bad start time", admin, `{"title":"x","date":"2026-12-01","startTime":"10am","endTime":"12:00"}`, nil, 400},
		{"missing end time", admin, `{"title":"x","date":"2026-12-01","startTime":"10:00"}`, nil, 400},
		{"duplicate id", admin, `{"id":"e1","title":"x","date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`, nil, 409},
		{"store error", admin, validEventJSON, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			s.createErr = tt.storeErr
			rec := do(t, eventsMux(s), "POST", "/api/events", tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 201 {
				if len(s.created) != 0 {
					t.Fatal("store must not be written")
				}
				return
			}
			got := decode[models.Event](t, rec)
			if got.ID == "" || got.Title != "New" {
				t.Fatalf("created = %+v", got)
			}
			if len(s.created) != 1 || s.created[0].ID != got.ID {
				t.Fatalf("store.created = %+v", s.created)
			}
		})
	}
}

func TestEventsCreate_KeepsProvidedID(t *testing.T) {
	s := newFakeEventStore()
	body := `{"id":"custom","title":"x","date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`
	rec := do(t, eventsMux(s), "POST", "/api/events", body, admin)
	if rec.Code != 201 || decode[models.Event](t, rec).ID != "custom" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestEventsUpdate(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		id         string
		body       string
		storeErr   error
		wantStatus int
	}{
		{"unauthenticated", anon, "e1", validEventJSON, nil, 401},
		{"consumer forbidden", consumer, "e1", validEventJSON, nil, 403},
		{"publisher updates", publisher, "e1", validEventJSON, nil, 200},
		{"admin updates", admin, "e1", validEventJSON, nil, 200},
		{"bad json", admin, "e1", `nope`, nil, 400},
		{"validation", admin, "e1", `{"title":""}`, nil, 400},
		{"not found", admin, "missing", validEventJSON, nil, 404},
		{"store error", admin, "e1", validEventJSON, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			s.err = tt.storeErr
			rec := do(t, eventsMux(s), "PUT", "/api/events/"+tt.id, tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == 200 {
				got := decode[models.Event](t, rec)
				if got.ID != tt.id || got.Title != "New" || s.events[tt.id].Title != "New" {
					t.Fatalf("updated = %+v", got)
				}
			}
		})
	}
}

func TestEventsUpdate_PathIDWins(t *testing.T) {
	s := newFakeEventStore(sampleEvents()...)
	body := `{"id":"e2","title":"New","date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`
	rec := do(t, eventsMux(s), "PUT", "/api/events/e1", body, admin)
	if rec.Code != 200 || s.events["e1"].Title != "New" || s.events["e2"].Title != "Hamlet" {
		t.Fatalf("status %d, e1=%q e2=%q", rec.Code, s.events["e1"].Title, s.events["e2"].Title)
	}
}

func TestEventsDelete(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		id         string
		storeErr   error
		wantStatus int
	}{
		{"unauthenticated", anon, "e1", nil, 401},
		{"consumer forbidden", consumer, "e1", nil, 403},
		{"publisher deletes", publisher, "e1", nil, 204},
		{"admin deletes", admin, "e1", nil, 204},
		{"not found", admin, "missing", nil, 404},
		{"store error", admin, "e1", errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			s.err = tt.storeErr
			rec := do(t, eventsMux(s), "DELETE", "/api/events/"+tt.id, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			_, stillThere := s.events["e1"]
			if wantGone := tt.wantStatus == 204; stillThere == wantGone {
				t.Fatalf("e1 still there = %v", stillThere)
			}
		})
	}
}
