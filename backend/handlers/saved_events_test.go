package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func savedMux(s store.UserSavedEventStore) *http.ServeMux {
	mux := http.NewServeMux()
	NewSavedEventsHandler(s).Register(mux, fakeProtect)
	return mux
}

// seededSavedStore: consumer saved e1; user-2 saved e2.
func seededSavedStore() *fakeSavedStore {
	s := newFakeSavedStore(sampleEvents()...)
	s.saved[savedKey{consumer.sub, "e1"}] = true
	s.saved[savedKey{"user-2", "e2"}] = true
	return s
}

func TestSavedEventsGet(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		storeErr   error
		wantStatus int
		wantIDs    []string
	}{
		{"unauthenticated", anon, nil, 401, nil},
		{"own saved events only", consumer, nil, 200, []string{"e1"}},
		{"nothing saved is []", admin, nil, 200, []string{}},
		{"not found", consumer, store.ErrNotFound, 404, nil},
		{"store error", consumer, errDB, 500, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := seededSavedStore()
			s.err = tt.storeErr
			rec := do(t, savedMux(s), "GET", "/api/savedEvents", "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus != 200 {
				return
			}
			got := decode[models.UserSavedEvents](t, rec)
			if got.UserId != tt.who.sub || got.SavedEvents == nil || len(got.SavedEvents) != len(tt.wantIDs) {
				t.Fatalf("got %+v, want ids %v", got, tt.wantIDs)
			}
			for i, id := range tt.wantIDs {
				if got.SavedEvents[i].ID != id {
					t.Fatalf("saved[%d] = %s, want %s", i, got.SavedEvents[i].ID, id)
				}
			}
		})
	}
}

func TestSavedEventsIsSaved(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		eventID    string
		storeErr   error
		wantStatus int
		wantBody   string
	}{
		{"unauthenticated", anon, "e1", nil, 401, ""},
		{"saved", consumer, "e1", nil, 200, `{"saved":true}` + "\n"},
		{"not saved", consumer, "e3", nil, 200, `{"saved":false}` + "\n"},
		{"saved by someone else only", consumer, "e2", nil, 200, `{"saved":false}` + "\n"},
		{"unknown event is false", consumer, "ghost", nil, 200, `{"saved":false}` + "\n"},
		{"store error", consumer, "e1", errDB, 500, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := seededSavedStore()
			s.err = tt.storeErr
			rec := do(t, savedMux(s), "GET", "/api/savedEvents/"+tt.eventID, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body, tt.wantBody)
			}
			if tt.wantStatus == 200 && rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestSavedEventsCreate(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		body       string
		createErr  error
		wantStatus int
	}{
		{"unauthenticated", anon, `{"eventId":"e3"}`, nil, 401},
		{"saves", consumer, `{"eventId":"e3"}`, nil, 200},
		{"userId in body is ignored", consumer, `{"eventId":"e3","userId":"user-2"}`, nil, 200},
		{"bad json", consumer, `{`, nil, 400},
		{"missing eventId", consumer, `{}`, nil, 400},
		{"already saved", consumer, `{"eventId":"e1"}`, nil, 409},
		{"unknown event", consumer, `{"eventId":"ghost"}`, nil, 404},
		{"store error", consumer, `{"eventId":"e3"}`, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := seededSavedStore()
			s.createErr = tt.createErr
			rec := do(t, savedMux(s), "POST", "/api/savedEvents", tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 200 {
				return
			}
			got := decode[models.UserSavedEvent](t, rec)
			if got.ID == "" || got.UserId != consumer.sub || got.EventId != "e3" {
				t.Fatalf("got %+v", got)
			}
			if !s.saved[savedKey{consumer.sub, "e3"}] || s.saved[savedKey{"user-2", "e3"}] {
				t.Fatalf("saved = %v", s.saved)
			}
		})
	}
}

func TestSavedEventsDelete(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		eventID    string
		storeErr   error
		wantStatus int
	}{
		{"unauthenticated", anon, "e1", nil, 401},
		{"deletes own", consumer, "e1", nil, 204},
		{"cannot delete someone else's", consumer, "e2", nil, 404},
		{"not saved", consumer, "e3", nil, 404},
		{"store error", consumer, "e1", errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := seededSavedStore()
			s.err = tt.storeErr
			rec := do(t, savedMux(s), "DELETE", "/api/savedEvents/"+tt.eventID, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !s.saved[savedKey{"user-2", "e2"}] {
				t.Fatal("other user's saved event was deleted")
			}
			if tt.wantStatus == 204 && s.saved[savedKey{consumer.sub, "e1"}] {
				t.Fatal("event still saved")
			}
		})
	}
}

// Handlers called without claims (e.g. mounted without protect) must 401
// rather than panic.
func TestHandlersWithoutClaims(t *testing.T) {
	sh := NewSavedEventsHandler(seededSavedStore())
	uh := NewUsersHandler(newFakeUserStore(sampleUsers()...))
	for name, h := range map[string]http.HandlerFunc{
		"saved get":    sh.get,
		"saved check":  sh.isSaved,
		"saved create": sh.create,
		"saved delete": sh.delete,
		"user update":  uh.update,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest("POST", "/", nil))
			if rec.Code != 401 {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}
