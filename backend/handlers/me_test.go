package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type meFixture struct {
	users    *fakeUserStore
	profiles *fakeProfileStore
	prefs    *fakePrefsStore
	events   *fakeEventStore
	mux      *http.ServeMux
}

func newMeFixture() *meFixture {
	f := &meFixture{
		users: newFakeUserStore(
			&models.User{ID: admin.sub, Email: "admin@x.com", Role: admin.role},
			&models.User{ID: publisher.sub, Email: "pub@x.com", Name: "Pub", Role: publisher.role},
			&models.User{ID: otherPublisher.sub, Email: "pub2@x.com", Role: otherPublisher.role},
			&models.User{ID: consumer.sub, Email: "user@x.com", Role: consumer.role},
		),
		profiles: newFakeProfileStore(&models.PublisherProfile{UserID: publisher.sub, OrgName: "Acme",
			Website: "https://acme.example", CreatedAt: "2026-11-01T10:00:00Z", UpdatedAt: "2026-11-02T10:00:00Z"}),
		prefs:  newFakePrefsStore(),
		events: newFakeEventStore(sampleEvents()...),
	}
	f.mux = http.NewServeMux()
	NewMeHandler(f.users, f.profiles, f.prefs, f.events).Register(f.mux, fakeProtect)
	return f
}

func errMsg(t *testing.T, rec interface{ Result() *http.Response }) string {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body["error"]
}

// ---- GET /api/me ----

func TestMeGet(t *testing.T) {
	stored := &models.NotificationPreferences{SavedEventReminders: false, CategoryNews: true,
		Categories: []models.EventCategory{models.CategoryArt}, Newsletter: true, PublisherActivity: false}

	tests := []struct {
		name        string
		who         identity
		setup       func(*meFixture)
		wantStatus  int
		wantProfile bool
		wantPrefs   *models.NotificationPreferences
	}{
		{name: "unauthenticated", who: anon, wantStatus: 401},
		{name: "consumer gets defaults and no profile", who: consumer, wantStatus: 200,
			wantPrefs: models.DefaultNotificationPreferences()},
		{name: "publisher with profile", who: publisher, wantStatus: 200, wantProfile: true,
			wantPrefs: models.DefaultNotificationPreferences()},
		{name: "publisher without profile", who: otherPublisher, wantStatus: 200,
			wantPrefs: models.DefaultNotificationPreferences()},
		{name: "admin", who: admin, wantStatus: 200, wantPrefs: models.DefaultNotificationPreferences()},
		{name: "stored prefs", who: consumer, wantStatus: 200, wantPrefs: stored,
			setup: func(f *meFixture) { f.prefs.prefs[consumer.sub] = stored }},
		{name: "consumer with a leftover profile row still gets null", who: consumer, wantStatus: 200,
			wantPrefs: models.DefaultNotificationPreferences(),
			setup:     func(f *meFixture) { f.profiles.profiles[consumer.sub] = &models.PublisherProfile{OrgName: "Old"} }},
		{name: "user not provisioned", who: identity{"ghost", "consumer"}, wantStatus: 404},
		{name: "user store error", who: consumer, wantStatus: 500,
			setup: func(f *meFixture) { f.users.err = errDB }},
		{name: "profile store error", who: publisher, wantStatus: 500,
			setup: func(f *meFixture) { f.profiles.err = errDB }},
		{name: "prefs store error", who: consumer, wantStatus: 500,
			setup: func(f *meFixture) { f.prefs.err = errDB }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMeFixture()
			if tt.setup != nil {
				tt.setup(f)
			}
			rec := do(t, f.mux, "GET", "/api/me", "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 200 {
				return
			}
			raw := decode[map[string]json.RawMessage](t, rec)
			for _, k := range []string{"user", "publisherProfile", "notifications"} {
				if _, ok := raw[k]; !ok {
					t.Fatalf("missing key %q in %s", k, rec.Body)
				}
			}
			resp := decode[meResponse](t, rec)
			if resp.User == nil || resp.User.ID != tt.who.sub {
				t.Fatalf("user = %+v", resp.User)
			}
			if tt.wantProfile {
				if resp.PublisherProfile == nil || resp.PublisherProfile.OrgName != "Acme" {
					t.Fatalf("profile = %+v", resp.PublisherProfile)
				}
			} else if string(raw["publisherProfile"]) != "null" {
				t.Fatalf("publisherProfile = %s, want null", raw["publisherProfile"])
			}
			if !reflect.DeepEqual(resp.Notifications, tt.wantPrefs) {
				t.Fatalf("notifications = %+v, want %+v", resp.Notifications, tt.wantPrefs)
			}
		})
	}
}

func TestMeGet_DefaultsJSON(t *testing.T) {
	rec := do(t, newMeFixture().mux, "GET", "/api/me", "", consumer)
	raw := decode[map[string]json.RawMessage](t, rec)
	want := `{"savedEventReminders":true,"categoryNews":false,"categories":[],"newsletter":false,"publisherActivity":true}`
	if string(raw["notifications"]) != want {
		t.Fatalf("notifications = %s\nwant %s", raw["notifications"], want)
	}
}

// ---- PUT /api/me/notifications ----

func TestMeUpdateNotifications(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		body       string
		upsertErr  error
		wantStatus int
		wantErr    string
		want       *models.NotificationPreferences
	}{
		{name: "unauthenticated", who: anon, body: `{}`, wantStatus: 401},
		{name: "consumer saves", who: consumer,
			body:       `{"savedEventReminders":false,"categoryNews":true,"categories":["Music","Art"],"newsletter":true,"publisherActivity":false}`,
			wantStatus: 200,
			want: &models.NotificationPreferences{CategoryNews: true, Newsletter: true,
				Categories: []models.EventCategory{"Music", "Art"}}},
		{name: "publisher saves", who: publisher, body: `{"publisherActivity":true}`, wantStatus: 200,
			want: &models.NotificationPreferences{PublisherActivity: true, Categories: []models.EventCategory{}}},
		{name: "admin saves", who: admin, body: `{"newsletter":true}`, wantStatus: 200,
			want: &models.NotificationPreferences{Newsletter: true, Categories: []models.EventCategory{}}},
		{name: "categories deduped in order", who: consumer,
			body: `{"categories":["Food","Music","Food","Music","Wellness"]}`, wantStatus: 200,
			want: &models.NotificationPreferences{Categories: []models.EventCategory{"Food", "Music", "Wellness"}}},
		{name: "null categories become []", who: consumer, body: `{"categories":null}`, wantStatus: 200,
			want: &models.NotificationPreferences{Categories: []models.EventCategory{}}},
		{name: "every category", who: consumer,
			body:       `{"categories":["Music","Theater","Parties","Sports","Food","Art","Wellness"]}`,
			wantStatus: 200, want: &models.NotificationPreferences{Categories: models.Categories}},
		{name: "unknown category", who: consumer, body: `{"categories":["Music","Cinema"]}`,
			wantStatus: 400, wantErr: `unknown category "Cinema"`},
		{name: "category is case sensitive", who: consumer, body: `{"categories":["music"]}`,
			wantStatus: 400, wantErr: "unknown category"},
		{name: "empty category", who: consumer, body: `{"categories":[""]}`,
			wantStatus: 400, wantErr: "unknown category"},
		{name: "bad json", who: consumer, body: `{"newsletter":`, wantStatus: 400, wantErr: "invalid request body"},
		{name: "wrong type", who: consumer, body: `{"newsletter":"yes"}`, wantStatus: 400, wantErr: "invalid request body"},
		{name: "user missing", who: consumer, body: `{}`, upsertErr: store.ErrNotFound, wantStatus: 404},
		{name: "store error", who: consumer, body: `{}`, upsertErr: errDB, wantStatus: 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMeFixture()
			f.prefs.upsertErr = tt.upsertErr
			rec := do(t, f.mux, "PUT", "/api/me/notifications", tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantErr != "" && !strings.Contains(errMsg(t, rec), tt.wantErr) {
				t.Fatalf("error = %s, want containing %q", rec.Body, tt.wantErr)
			}
			if tt.wantStatus != 200 {
				if len(f.prefs.upserted) != 0 && tt.upsertErr == nil {
					t.Fatal("store must not be written")
				}
				return
			}
			got := decode[models.NotificationPreferences](t, rec)
			if !reflect.DeepEqual(&got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, *tt.want)
			}
			if stored := f.prefs.prefs[tt.who.sub]; !reflect.DeepEqual(stored, tt.want) {
				t.Fatalf("stored for %s = %+v", tt.who.sub, stored)
			}
		})
	}
}

func TestMeUpdateNotifications_EmptyCategoriesJSON(t *testing.T) {
	rec := do(t, newMeFixture().mux, "PUT", "/api/me/notifications", `{}`, consumer)
	if !strings.Contains(rec.Body.String(), `"categories":[]`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// ---- GET /api/me/publisher-profile ----

func TestMeGetPublisherProfile(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		storeErr   error
		wantStatus int
	}{
		{"unauthenticated", anon, nil, 401},
		{"consumer forbidden", consumer, nil, 403},
		{"admin forbidden", admin, nil, 403},
		{"found", publisher, nil, 200},
		{"not created yet", otherPublisher, nil, 404},
		{"store error", publisher, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMeFixture()
			f.profiles.err = tt.storeErr
			rec := do(t, f.mux, "GET", "/api/me/publisher-profile", "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == 200 {
				want := `{"userId":"pub-1","orgName":"Acme","website":"https://acme.example",` +
					`"createdAt":"2026-11-01T10:00:00Z","updatedAt":"2026-11-02T10:00:00Z"}`
				if got := strings.TrimSpace(rec.Body.String()); got != want {
					t.Fatalf("body = %s\nwant %s", got, want)
				}
			}
		})
	}
}

// ---- PUT /api/me/publisher-profile ----

func TestMePutPublisherProfile(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name       string
		who        identity
		body       string
		upsertErr  error
		wantStatus int
		wantErr    string
		want       *models.PublisherProfile // fields sent to the store
	}{
		{name: "unauthenticated", who: anon, body: `{"orgName":"Acme"}`, wantStatus: 401},
		{name: "consumer forbidden", who: consumer, body: `{"orgName":"Acme"}`, wantStatus: 403},
		{name: "admin forbidden", who: admin, body: `{"orgName":"Acme"}`, wantStatus: 403},
		{name: "creates minimal", who: otherPublisher, body: `{"orgName":"Acme"}`, wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-2", OrgName: "Acme"}},
		{name: "replaces full, trimmed", who: publisher,
			body:       `{"orgName":"  Acme Events ","website":" https://acme.example/about ","phone":" +34 (985) 123-456.7 ","description":"  We host things.  "}`,
			wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-1", OrgName: "Acme Events", Website: "https://acme.example/about",
				Phone: "+34 (985) 123-456.7", Description: "We host things."}},
		{name: "blank optionals stored empty", who: publisher,
			body: `{"orgName":"Acme","website":"   ","phone":"","description":" "}`, wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-1", OrgName: "Acme"}},
		{name: "null optionals", who: publisher,
			body: `{"orgName":"Acme","website":null,"phone":null,"description":null}`, wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-1", OrgName: "Acme"}},
		{name: "http website", who: publisher, body: `{"orgName":"Acme","website":"http://acme.example"}`,
			wantStatus: 200, want: &models.PublisherProfile{UserID: "pub-1", OrgName: "Acme", Website: "http://acme.example"}},
		{name: "uppercase scheme", who: publisher, body: `{"orgName":"Acme","website":"HTTPS://acme.example"}`,
			wantStatus: 200, want: &models.PublisherProfile{UserID: "pub-1", OrgName: "Acme", Website: "HTTPS://acme.example"}},
		{name: "orgName at min", who: publisher, body: `{"orgName":"AB"}`, wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-1", OrgName: "AB"}},
		{name: "orgName at max", who: publisher, body: `{"orgName":"` + long(120) + `"}`, wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-1", OrgName: long(120)}},
		{name: "orgName counts characters, not bytes", who: publisher, body: `{"orgName":"` + strings.Repeat("ñ", 120) + `"}`,
			wantStatus: 200, want: &models.PublisherProfile{UserID: "pub-1", OrgName: strings.Repeat("ñ", 120)}},
		{name: "limits at max", who: publisher,
			body: `{"orgName":"Acme","website":"https://a.example/` + long(300-len("https://a.example/")) +
				`","phone":"` + strings.Repeat("1", 30) + `","description":"` + long(1000) + `"}`,
			wantStatus: 200,
			want: &models.PublisherProfile{UserID: "pub-1", OrgName: "Acme",
				Website: "https://a.example/" + long(300-len("https://a.example/")),
				Phone:   strings.Repeat("1", 30), Description: long(1000)}},

		{name: "bad json", who: publisher, body: `{"orgName":`, wantStatus: 400, wantErr: "invalid request body"},
		{name: "missing orgName", who: publisher, body: `{}`, wantStatus: 400, wantErr: "orgName"},
		{name: "orgName only spaces", who: publisher, body: `{"orgName":"    "}`, wantStatus: 400, wantErr: "orgName"},
		{name: "orgName too short after trim", who: publisher, body: `{"orgName":" A "}`, wantStatus: 400, wantErr: "orgName"},
		{name: "orgName too long", who: publisher, body: `{"orgName":"` + long(121) + `"}`, wantStatus: 400, wantErr: "orgName"},
		{name: "website relative", who: publisher, body: `{"orgName":"Acme","website":"acme.example"}`,
			wantStatus: 400, wantErr: "website"},
		{name: "website ftp", who: publisher, body: `{"orgName":"Acme","website":"ftp://acme.example"}`,
			wantStatus: 400, wantErr: "website"},
		{name: "website without host", who: publisher, body: `{"orgName":"Acme","website":"https://"}`,
			wantStatus: 400, wantErr: "website"},
		{name: "website javascript", who: publisher, body: `{"orgName":"Acme","website":"javascript:alert(1)"}`,
			wantStatus: 400, wantErr: "website"},
		{name: "website unparsable", who: publisher, body: `{"orgName":"Acme","website":"http://[::1"}`,
			wantStatus: 400, wantErr: "website"},
		{name: "website too long", who: publisher,
			body:       `{"orgName":"Acme","website":"https://a.example/` + long(301-len("https://a.example/")) + `"}`,
			wantStatus: 400, wantErr: "website must be at most 300"},
		{name: "phone too long", who: publisher, body: `{"orgName":"Acme","phone":"` + strings.Repeat("1", 31) + `"}`,
			wantStatus: 400, wantErr: "phone must be at most 30"},
		{name: "phone letters", who: publisher, body: `{"orgName":"Acme","phone":"call me"}`,
			wantStatus: 400, wantErr: "phone may only contain"},
		{name: "phone other symbols", who: publisher, body: `{"orgName":"Acme","phone":"123#4"}`,
			wantStatus: 400, wantErr: "phone may only contain"},
		{name: "description too long", who: publisher, body: `{"orgName":"Acme","description":"` + long(1001) + `"}`,
			wantStatus: 400, wantErr: "description"},

		{name: "user missing", who: publisher, body: `{"orgName":"Acme"}`, upsertErr: store.ErrNotFound, wantStatus: 404},
		{name: "store error", who: publisher, body: `{"orgName":"Acme"}`, upsertErr: errDB, wantStatus: 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMeFixture()
			f.profiles.upsertErr = tt.upsertErr
			rec := do(t, f.mux, "PUT", "/api/me/publisher-profile", tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantErr != "" && !strings.Contains(errMsg(t, rec), tt.wantErr) {
				t.Fatalf("error = %s, want containing %q", rec.Body, tt.wantErr)
			}
			if tt.wantStatus != 200 {
				if len(f.profiles.upserted) != 0 {
					t.Fatal("store must not be written")
				}
				return
			}
			if len(f.profiles.upserted) != 1 {
				t.Fatalf("upserted = %+v", f.profiles.upserted)
			}
			sent := f.profiles.upserted[0]
			sent.CreatedAt, sent.UpdatedAt = "", ""
			if !reflect.DeepEqual(&sent, tt.want) {
				t.Fatalf("sent  %+v\nwant %+v", sent, *tt.want)
			}
			got := decode[models.PublisherProfile](t, rec)
			if got.UserID != tt.who.sub || got.OrgName != tt.want.OrgName || got.CreatedAt == "" || got.UpdatedAt == "" {
				t.Fatalf("response = %+v", got)
			}
		})
	}
}

func TestMePutPublisherProfile_UpdateKeepsCreatedAt(t *testing.T) {
	f := newMeFixture()
	rec := do(t, f.mux, "PUT", "/api/me/publisher-profile", `{"orgName":"Acme 2"}`, publisher)
	got := decode[models.PublisherProfile](t, rec)
	if rec.Code != 200 || got.OrgName != "Acme 2" || got.CreatedAt != "2026-11-01T10:00:00Z" || got.Website != "" {
		t.Fatalf("status %d got %+v", rec.Code, got)
	}
	// The PUT is a full replacement: website was cleared.
	if strings.Contains(rec.Body.String(), "website") {
		t.Fatalf("empty website must be omitted: %s", rec.Body)
	}
}

// ---- GET /api/me/events ----

func TestMeListEvents(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		storeErr   error
		wantStatus int
		wantIDs    string
	}{
		{"unauthenticated", anon, nil, 401, ""},
		{"consumer forbidden", consumer, nil, 403, ""},
		{"admin forbidden", admin, nil, 403, ""},
		{"own events only, by start time", publisher, nil, 200, "e0,e1,e4"},
		{"other publisher", otherPublisher, nil, 200, "e2"},
		{"publisher without events", identity{"pub-3", "publisher"}, nil, 200, ""},
		{"store error", publisher, errDB, 500, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMeFixture()
			// e0 is in the past, e4 later than e1: all of them are listed.
			f.events.events["e0"] = &models.Event{ID: "e0", Title: "Past", Date: "2020-01-01", StartTime: "10:00",
				PublisherID: publisher.sub}
			f.events.events["e4"] = &models.Event{ID: "e4", Title: "Later", Date: "2027-06-01", StartTime: "09:00",
				PublisherID: publisher.sub}
			f.events.saves = map[string]int{"e1": 3, "e2": 1}
			f.events.err = tt.storeErr
			rec := do(t, f.mux, "GET", "/api/me/events", "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 200 {
				return
			}
			if f.events.lastPublisher != tt.who.sub {
				t.Fatalf("listed for %q, want token subject %q", f.events.lastPublisher, tt.who.sub)
			}
			resp := decode[publisherEventsResponse](t, rec)
			if resp.Events == nil || resp.Total != len(resp.Events) {
				t.Fatalf("resp = %+v", resp)
			}
			var got []string
			for _, e := range resp.Events {
				got = append(got, e.ID)
				if e.SavesCount != f.events.saves[e.ID] {
					t.Errorf("%s savesCount = %d, want %d", e.ID, e.SavesCount, f.events.saves[e.ID])
				}
			}
			if strings.Join(got, ",") != tt.wantIDs {
				t.Fatalf("ids = %v, want %s", got, tt.wantIDs)
			}
		})
	}
}

func TestMeListEvents_JSONShape(t *testing.T) {
	f := newMeFixture()
	f.events = newFakeEventStore()
	f.mux = http.NewServeMux()
	NewMeHandler(f.users, f.profiles, f.prefs, f.events).Register(f.mux, fakeProtect)
	rec := do(t, f.mux, "GET", "/api/me/events", "", publisher)
	if got := strings.TrimSpace(rec.Body.String()); got != `{"events":[],"total":0}` {
		t.Fatalf("empty body = %s", got)
	}

	f.events.events["e1"] = &models.Event{ID: "e1", Title: "T", PublisherID: publisher.sub}
	rec = do(t, f.mux, "GET", "/api/me/events", "", publisher)
	if !strings.Contains(rec.Body.String(), `"savesCount":0`) || !strings.Contains(rec.Body.String(), `"publisherId":"pub-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}
