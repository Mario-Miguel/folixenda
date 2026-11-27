package handlers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/models"
)

// body builds an event JSON body with the given extra fields and sessions.
func eventBody(extra string, sessions ...string) string {
	if len(sessions) == 0 {
		sessions = []string{`{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`}
	}
	if extra != "" {
		extra += ","
	}
	return `{` + extra + `"sessions":[` + strings.Join(sessions, ",") + `]}`
}

func TestPrepareEventValidation(t *testing.T) {
	base := `"title":"T","venue":"V"`
	ok := `{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`
	many := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf(`{"date":"2026-12-01","startTime":"%02d:%02d","endTime":"23:59"}`, i/60, i%60)
		}
		return out
	}
	perks := func(n, size int) string {
		p := make([]string, n)
		for i := range p {
			p[i] = `"` + strings.Repeat("p", size) + `"`
		}
		return `"perks":[` + strings.Join(p, ",") + `]`
	}
	tests := []struct {
		name    string
		body    string
		wantErr string // "" = valid
	}{
		{"valid", eventBody(base), ""},
		{"title trimmed to empty", eventBody(`"title":"   ","venue":"V"`), "title is required"},
		{"title 200 chars ok", eventBody(`"venue":"V","title":"` + strings.Repeat("é", 200) + `"`), ""},
		{"title 201 chars", eventBody(`"venue":"V","title":"` + strings.Repeat("é", 201) + `"`), "title must be at most 200"},
		{"description 5000 ok", eventBody(base + `,"description":"` + strings.Repeat("d", 5000) + `"`), ""},
		{"description 5001", eventBody(base + `,"description":"` + strings.Repeat("d", 5001) + `"`), "description must be at most 5000"},
		{"venue required", eventBody(`"title":"T","venue":"  "`), "venue is required"},
		{"price 0 ok", eventBody(base + `,"price":0`), ""},
		{"negative price", eventBody(base + `,"price":-0.5`), "price must be 0 or more"},
		{"20 perks of 80 ok", eventBody(base + "," + perks(20, 80)), ""},
		{"21 perks", eventBody(base + "," + perks(21, 1)), "perks must have at most 20"},
		{"perk too long", eventBody(base + "," + perks(1, 81)), "perks[0] must be at most 80"},
		{"no sessions", `{` + base + `,"sessions":[]}`, "sessions must have between 1 and 60"},
		{"sessions missing", `{` + base + `}`, "sessions must have between 1 and 60"},
		{"60 sessions ok", eventBody(base, many(60)...), ""},
		{"61 sessions", eventBody(base, many(61)...), "sessions must have between 1 and 60"},
		{"bad date", eventBody(base, `{"date":"01-12-2026","startTime":"10:00","endTime":"12:00"}`), "sessions[0].date"},
		{"impossible date", eventBody(base, ok, `{"date":"2026-02-30","startTime":"10:00","endTime":"12:00"}`), "sessions[1].date"},
		{"bad start", eventBody(base, `{"date":"2026-12-01","startTime":"10am","endTime":"12:00"}`), "sessions[0].startTime"},
		{"single-digit hour", eventBody(base, `{"date":"2026-12-01","startTime":"9:00","endTime":"12:00"}`), "sessions[0].startTime"},
		{"missing end", eventBody(base, `{"date":"2026-12-01","startTime":"10:00"}`), "sessions[0].endTime"},
		{"duplicate date+start", eventBody(base, ok, `{"date":"2026-12-01","startTime":"10:00","endTime":"13:00"}`), "duplicates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, eventsMux(newFakeEventStore()), "POST", "/api/events", tt.body, admin)
			if tt.wantErr == "" {
				if rec.Code != 201 {
					t.Fatalf("status = %d (%s)", rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != 400 || !strings.Contains(decode[map[string]string](t, rec)["error"], tt.wantErr) {
				t.Fatalf("status = %d body %s, want 400 %q", rec.Code, rec.Body, tt.wantErr)
			}
		})
	}
}

func TestEventsCreate_MultiDay(t *testing.T) {
	s := newFakeEventStore()
	body := eventBody(`"title":"Fest","venue":"V","date":"1999-01-01","startTime":"00:00","endTime":"00:01","endDate":"1999-01-02"`,
		`{"date":"2026-12-03","startTime":"22:00","endTime":"02:00"}`,
		`{"date":"2026-12-01","startTime":"18:00","endTime":"20:00"}`,
		`{"date":"2026-12-01","startTime":"10:00","endTime":"12:00"}`)
	rec := do(t, eventsMux(s), "POST", "/api/events", body, publisher)
	if rec.Code != 201 {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	got := decode[models.Event](t, rec)
	if got.Date != "2026-12-01" || got.StartTime != "10:00" || got.EndTime != "12:00" || got.EndDate != "2026-12-03" {
		t.Fatalf("derived fields from body, not sessions: %+v", got)
	}
	if len(got.Sessions) != 3 || got.Sessions[1].StartTime != "18:00" || got.Sessions[2].EndTime != "02:00" {
		t.Fatalf("sessions not sorted: %+v", got.Sessions)
	}
	if got.Images == nil || !strings.Contains(rec.Body.String(), `"images":[]`) {
		t.Fatalf("images must be []: %s", rec.Body)
	}

	// The list filter matches any session, not just the first.
	rec = do(t, eventsMux(s), "GET", "/api/events?date=2026-12-03", "", anon)
	if resp := decode[listResponse](t, rec); resp.Total != 1 || resp.Events[0].ID != got.ID {
		t.Fatalf("date filter on a later session: %s", rec.Body)
	}
}

func TestEventsImageURL(t *testing.T) {
	withCover := func() *fakeEventStore {
		s := newFakeEventStore(sampleEvents()...)
		e := s.events["e3"]
		e.Images = []models.EventImage{{ID: "c", Kind: models.ImageKindCover, URL: "http://cdn/c.jpg", ObjectKey: "c.jpg"}}
		e.ApplyImages()
		return s
	}
	t.Run("create stores the external imageUrl", func(t *testing.T) {
		s := newFakeEventStore()
		rec := do(t, eventsMux(s), "POST", "/api/events", eventBody(`"title":"T","venue":"V","imageUrl":"https://x.example/a.png"`), admin)
		if rec.Code != 201 || s.created[0].StoredImageURL != "https://x.example/a.png" ||
			decode[models.Event](t, rec).ImageURL != "https://x.example/a.png" {
			t.Fatalf("status %d created %+v", rec.Code, s.created)
		}
	})
	t.Run("update with a cover ignores imageUrl (even an invalid one)", func(t *testing.T) {
		for _, url := range []string{"http://cdn/c.jpg", "not a url", ""} {
			s := withCover()
			rec := do(t, eventsMux(s), "PUT", "/api/events/e3", eventBody(`"title":"T","venue":"V","imageUrl":"`+url+`"`), admin)
			if rec.Code != 200 {
				t.Fatalf("%q: status %d %s", url, rec.Code, rec.Body)
			}
			if got := s.events["e3"].StoredImageURL; got != "https://scraped.example/rock.jpg" {
				t.Fatalf("%q: stored image_url = %q", url, got)
			}
			if got := decode[models.Event](t, rec); got.ImageURL != "http://cdn/c.jpg" || len(got.Images) != 1 {
				t.Fatalf("%q: response %+v", url, got)
			}
		}
	})
	t.Run("update without a cover stores imageUrl", func(t *testing.T) {
		s := newFakeEventStore(sampleEvents()...)
		rec := do(t, eventsMux(s), "PUT", "/api/events/e3", eventBody(`"title":"T","venue":"V","imageUrl":""`), admin)
		if rec.Code != 200 || s.events["e3"].StoredImageURL != "" {
			t.Fatalf("status %d stored %q", rec.Code, s.events["e3"].StoredImageURL)
		}
	})
	t.Run("update without a cover rejects a bad imageUrl", func(t *testing.T) {
		s := newFakeEventStore(sampleEvents()...)
		rec := do(t, eventsMux(s), "PUT", "/api/events/e3", eventBody(`"title":"T","venue":"V","imageUrl":"ftp://x/y"`), admin)
		if rec.Code != 400 || s.events["e3"].Title != "Rock" {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func TestEventsWrite_ReloadFallback(t *testing.T) {
	// The write succeeds but re-reading fails: respond with what was written.
	s := newFakeEventStore()
	s.failGetsAfter(1)
	rec := do(t, eventsMux(s), "POST", "/api/events", eventBody(`"title":"T","venue":"V","imageUrl":"https://x/a.png"`), admin)
	if rec.Code != 201 {
		t.Fatalf("status %d", rec.Code)
	}
	if got := decode[models.Event](t, rec); got.Title != "T" || got.ImageURL != "https://x/a.png" || got.Images == nil {
		t.Fatalf("got %+v", got)
	}
}

func TestEventsDelete_RemovesObjects(t *testing.T) {
	s := newFakeEventStore(sampleEvents()...)
	s.events["e1"].Images = []models.EventImage{{ID: "a", ObjectKey: "events/e1/a.jpg"}, {ID: "b", ObjectKey: "events/e1/b.png"}}
	objects := newFakeObjects()
	objects.deleteErr = errDB // best effort: still 204
	rec := do(t, eventsMuxWith(s, objects), "DELETE", "/api/events/e1", "", publisher)
	if rec.Code != 204 || strings.Join(objects.deleted, ",") != "events/e1/a.jpg,events/e1/b.png" {
		t.Fatalf("status %d deleted %v", rec.Code, objects.deleted)
	}
}
