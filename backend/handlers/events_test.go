package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func sampleEvents() []*models.Event {
	events := []*models.Event{
		// e1 is owned by publisher (pub-1), e2 by otherPublisher (pub-2), e3 is scraped (no owner).
		{ID: "e1", Title: "Jazz", Category: models.CategoryMusic, Date: "2026-11-01", StartTime: "20:00", EndTime: "22:00",
			PublisherID: publisher.sub},
		{ID: "e2", Title: "Hamlet", Category: models.CategoryTheater, Date: "2026-11-05", StartTime: "19:00", EndTime: "21:00",
			PublisherID: otherPublisher.sub},
		{ID: "e3", Title: "Rock", Category: models.CategoryMusic, Date: "2026-11-10", StartTime: "21:00", EndTime: "23:30",
			StoredImageURL: "https://scraped.example/rock.jpg"},
	}
	for _, e := range events {
		e.Sessions = []models.EventSession{{Date: e.Date, StartTime: e.StartTime, EndTime: e.EndTime}}
		e.ApplySessions()
		e.ApplyImages()
	}
	return events
}

var otherPublisher = identity{"pub-2", auth.RolePublisher}

func eventsMux(s *fakeEventStore) *http.ServeMux {
	return eventsMuxWith(s, newFakeObjects())
}

func eventsMuxWith(s *fakeEventStore, objects *fakeObjects) *http.ServeMux {
	mux := http.NewServeMux()
	NewEventsHandler(s, &fakeImageStore{events: s}, objects).Register(mux, fakeProtect)
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

const validEventJSON = `{"title":"New","category":"Art","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`

func TestEventsCreate(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		body       string
		storeErr   error
		wantStatus int
		wantOwner  string
	}{
		{"unauthenticated", anon, validEventJSON, nil, 401, ""},
		{"consumer forbidden", consumer, validEventJSON, nil, 403, ""},
		{"no role forbidden", identity{"x", ""}, validEventJSON, nil, 403, ""},
		{"publisher creates and owns it", publisher, validEventJSON, nil, 201, publisher.sub},
		{"publisher cannot create for someone else", publisher,
			`{"publisherId":"pub-2","title":"New","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`,
			nil, 201, publisher.sub},
		{"admin creates unowned", admin, validEventJSON, nil, 201, ""},
		{"admin creates for a publisher", admin,
			`{"publisherId":"pub-2","title":"New","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`,
			nil, 201, "pub-2"},
		{"bad json", admin, `{"title":`, nil, 400, ""},
		{"missing title", admin, `{"venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`, nil, 400, ""},
		{"legacy body without sessions", admin, `{"title":"x","venue":"V","date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`, nil, 400, ""},
		{"bad imageUrl", admin, `{"title":"x","venue":"V","imageUrl":"javascript:alert(1)","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`, nil, 400, ""},
		{"duplicate id", admin, `{"id":"e1","title":"x","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`, nil, 409, ""},
		{"unknown publisherId", admin, validEventJSON, store.ErrNotFound, 400, ""},
		{"store error", admin, validEventJSON, errDB, 500, ""},
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
			if got.ID == "" || got.Title != "New" || got.PublisherID != tt.wantOwner {
				t.Fatalf("created = %+v, want owner %q", got, tt.wantOwner)
			}
			if len(s.created) != 1 || s.created[0].ID != got.ID || s.created[0].PublisherID != tt.wantOwner {
				t.Fatalf("store.created = %+v", s.created)
			}
		})
	}
}

func TestEventsCreate_GatedByCanPublish(t *testing.T) {
	for _, who := range []identity{admin, publisher, consumer, {"x", "root"}} {
		s := newFakeEventStore()
		rec := do(t, eventsMux(s), "POST", "/api/events", validEventJSON, who)
		want := 403
		if auth.CanPublish(&auth.Claims{Role: who.role}) {
			want = 201
		}
		if rec.Code != want {
			t.Errorf("role %q: status = %d, want %d", who.role, rec.Code, want)
		}
	}
}

func TestEventsCreate_KeepsProvidedID(t *testing.T) {
	s := newFakeEventStore()
	body := `{"id":"custom","title":"x","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`
	rec := do(t, eventsMux(s), "POST", "/api/events", body, admin)
	if rec.Code != 201 || decode[models.Event](t, rec).ID != "custom" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestEventsCreate_IgnoresOrganizer(t *testing.T) {
	s := newFakeEventStore()
	body := `{"title":"x","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}],"organizer":{"id":"x","orgName":"Fake"}}`
	rec := do(t, eventsMux(s), "POST", "/api/events", body, publisher)
	if rec.Code != 201 || s.created[0].Organizer != nil {
		t.Fatalf("status %d created %+v", rec.Code, s.created)
	}
}

func TestEventsUpdate(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		id         string
		body       string
		getErr     error
		updateErr  error
		wantStatus int
	}{
		{"unauthenticated", anon, "e1", validEventJSON, nil, nil, 401},
		{"consumer forbidden", consumer, "e1", validEventJSON, nil, nil, 403},
		{"publisher updates own event", publisher, "e1", validEventJSON, nil, nil, 200},
		{"publisher cannot update another publisher's event", publisher, "e2", validEventJSON, nil, nil, 403},
		{"publisher cannot update a scraped event", publisher, "e3", validEventJSON, nil, nil, 403},
		{"admin updates any publisher's event", admin, "e2", validEventJSON, nil, nil, 200},
		{"admin updates a scraped event", admin, "e3", validEventJSON, nil, nil, 200},
		{"bad json", publisher, "e1", `nope`, nil, nil, 400},
		{"validation", admin, "e1", `{"title":""}`, nil, nil, 400},
		{"not found", admin, "missing", validEventJSON, nil, nil, 404},
		{"not found for publisher too", publisher, "missing", validEventJSON, nil, nil, 404},
		{"get error", admin, "e1", validEventJSON, errDB, nil, 500},
		{"deleted meanwhile", admin, "e1", validEventJSON, nil, store.ErrNotFound, 404},
		{"update error", admin, "e1", validEventJSON, nil, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			before := map[string]string{}
			for id, e := range s.events {
				before[id] = e.Title
			}
			s.err = tt.getErr
			s.updateErr = tt.updateErr
			rec := do(t, eventsMux(s), "PUT", "/api/events/"+tt.id, tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == 200 {
				got := decode[models.Event](t, rec)
				if got.ID != tt.id || got.Title != "New" || s.events[tt.id].Title != "New" {
					t.Fatalf("updated = %+v", got)
				}
				return
			}
			for id, e := range s.events {
				if e.Title != before[id] {
					t.Fatalf("event %s changed to %q", id, e.Title)
				}
			}
		})
	}
}

func TestEventsUpdate_KeepsPublisherID(t *testing.T) {
	body := `{"publisherId":"intruder","title":"New","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`
	for _, tt := range []struct {
		who       identity
		id, owner string
	}{
		{publisher, "e1", publisher.sub},
		{admin, "e2", otherPublisher.sub},
		{admin, "e3", ""},
	} {
		s := newFakeEventStore(sampleEvents()...)
		rec := do(t, eventsMux(s), "PUT", "/api/events/"+tt.id, body, tt.who)
		if rec.Code != 200 {
			t.Fatalf("%s: status %d (%s)", tt.id, rec.Code, rec.Body)
		}
		if got := decode[models.Event](t, rec).PublisherID; got != tt.owner {
			t.Errorf("%s: response publisherId = %q, want %q", tt.id, got, tt.owner)
		}
		if got := s.events[tt.id].PublisherID; got != tt.owner {
			t.Errorf("%s: stored publisherId = %q, want %q", tt.id, got, tt.owner)
		}
	}
}

func TestEventsUpdate_PathIDWins(t *testing.T) {
	s := newFakeEventStore(sampleEvents()...)
	body := `{"id":"e2","title":"New","venue":"V","sessions":[{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}]}`
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
		getErr     error
		deleteErr  error
		wantStatus int
	}{
		{"unauthenticated", anon, "e1", nil, nil, 401},
		{"consumer forbidden", consumer, "e1", nil, nil, 403},
		{"publisher deletes own event", publisher, "e1", nil, nil, 204},
		{"publisher cannot delete another publisher's event", publisher, "e2", nil, nil, 403},
		{"publisher cannot delete a scraped event", publisher, "e3", nil, nil, 403},
		{"admin deletes any publisher's event", admin, "e2", nil, nil, 204},
		{"admin deletes a scraped event", admin, "e3", nil, nil, 204},
		{"not found", admin, "missing", nil, nil, 404},
		{"get error", admin, "e1", errDB, nil, 500},
		{"deleted meanwhile", admin, "e1", nil, store.ErrNotFound, 404},
		{"delete error", admin, "e1", nil, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeEventStore(sampleEvents()...)
			s.err = tt.getErr
			s.deleteErr = tt.deleteErr
			rec := do(t, eventsMux(s), "DELETE", "/api/events/"+tt.id, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			wantDeleted := ""
			if tt.wantStatus == 204 {
				wantDeleted = tt.id
			}
			if got := strings.Join(s.deleted, ","); got != wantDeleted {
				t.Fatalf("deleted = %q, want %q", got, wantDeleted)
			}
		})
	}
}

func TestOwns(t *testing.T) {
	owned := &models.Event{PublisherID: "pub-1"}
	scraped := &models.Event{}
	for _, tt := range []struct {
		name   string
		claims *auth.Claims
		e      *models.Event
		want   bool
	}{
		{"admin, owned by other", claimsOf(admin), owned, true},
		{"admin, scraped", claimsOf(admin), scraped, true},
		{"owner", claimsOf(publisher), owned, true},
		{"other publisher", claimsOf(otherPublisher), owned, false},
		{"publisher, scraped", claimsOf(publisher), scraped, false},
		{"publisher without subject, scraped", claimsOf(identity{"", auth.RolePublisher}), scraped, false},
		{"consumer with the owner's id", claimsOf(identity{"pub-1", auth.RoleConsumer}), owned, false},
	} {
		if got := owns(tt.claims, tt.e); got != tt.want {
			t.Errorf("%s: owns = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEventsGet_OrganizerJSON(t *testing.T) {
	e := &models.Event{ID: "e9", Title: "T", PublisherID: "pub-1",
		Organizer: &models.Organizer{ID: "pub-1", OrgName: "Acme", Website: "https://acme.example"}}
	rec := do(t, eventsMux(newFakeEventStore(e, &models.Event{ID: "e8", Title: "Scraped"})), "GET", "/api/events/e9", "", anon)
	body := rec.Body.String()
	if !strings.Contains(body, `"publisherId":"pub-1"`) ||
		!strings.Contains(body, `"organizer":{"id":"pub-1","orgName":"Acme","website":"https://acme.example"}`) {
		t.Fatalf("body = %s", body)
	}
	rec = do(t, eventsMux(newFakeEventStore(&models.Event{ID: "e8", Title: "Scraped"})), "GET", "/api/events/e8", "", anon)
	if strings.Contains(rec.Body.String(), "organizer") || strings.Contains(rec.Body.String(), "publisherId") {
		t.Fatalf("scraped event body = %s", rec.Body)
	}
}
