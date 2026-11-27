package handlers

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

func sampleUsers() []*models.User {
	return []*models.User{
		{ID: admin.sub, Email: "admin@example.com", Name: "Admin", Role: "admin"},
		{ID: consumer.sub, Email: "user@example.com", Name: "User", Role: "consumer",
			SubscriptionType: "free", EventPreferences: []string{"Music"}},
		{ID: "user-2", Email: "other@example.com", Name: "Other", Role: "consumer"},
	}
}

func usersMux(s store.UserStore) *http.ServeMux {
	mux := http.NewServeMux()
	NewUsersHandler(s).Register(mux, fakeProtect)
	return mux
}

func TestUsersList(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		users      []*models.User
		storeErr   error
		wantStatus int
		wantLen    int
	}{
		{"unauthenticated", anon, sampleUsers(), nil, 401, 0},
		{"consumer forbidden", consumer, sampleUsers(), nil, 403, 0},
		{"publisher forbidden", publisher, sampleUsers(), nil, 403, 0},
		{"admin lists", admin, sampleUsers(), nil, 200, 3},
		{"empty is []", admin, nil, nil, 200, 0},
		{"store error", admin, sampleUsers(), errDB, 500, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeUserStore(tt.users...)
			s.err = tt.storeErr
			rec := do(t, usersMux(s), "GET", "/api/users", "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == 200 {
				if rec.Body.String() == "null\n" {
					t.Fatal("must encode [] not null")
				}
				if got := decode[[]models.User](t, rec); len(got) != tt.wantLen {
					t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
				}
			}
		})
	}
}

func TestUsersGet(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		id         string
		storeErr   error
		wantStatus int
	}{
		{"unauthenticated", anon, consumer.sub, nil, 401},
		{"self", consumer, consumer.sub, nil, 200},
		{"other user forbidden", consumer, "user-2", nil, 403},
		{"publisher on other forbidden", publisher, consumer.sub, nil, 403},
		{"admin on other", admin, consumer.sub, nil, 200},
		{"admin not found", admin, "ghost", nil, 404},
		{"store error", admin, consumer.sub, errDB, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeUserStore(sampleUsers()...)
			s.err = tt.storeErr
			rec := do(t, usersMux(s), "GET", "/api/users/"+tt.id, "", tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == 200 && decode[models.User](t, rec).Email != "user@example.com" {
				t.Fatalf("body = %s", rec.Body)
			}
		})
	}
}

func TestUsersUpdate(t *testing.T) {
	tests := []struct {
		name       string
		who        identity
		id         string
		body       string
		storeErr   error
		updateErr  error
		wantStatus int
		want       *models.User // stored user after a successful update
	}{
		{name: "unauthenticated", who: anon, id: consumer.sub, body: `{"name":"x"}`, wantStatus: 401},
		{name: "other user forbidden", who: consumer, id: "user-2", body: `{"name":"x"}`, wantStatus: 403},
		{name: "publisher on other forbidden", who: publisher, id: consumer.sub, body: `{"name":"x"}`, wantStatus: 403},
		{name: "self partial update keeps other fields", who: consumer, id: consumer.sub,
			body: `{"name":"New Name","paymentMethod":"card"}`, wantStatus: 200,
			want: &models.User{ID: consumer.sub, Email: "user@example.com", Name: "New Name", Role: "consumer",
				SubscriptionType: "free", PaymentMethod: "card", EventPreferences: []string{"Music"}}},
		{name: "self cannot change own role (ignored)", who: consumer, id: consumer.sub,
			body: `{"role":"admin","name":"Sneaky"}`, wantStatus: 200,
			want: &models.User{ID: consumer.sub, Email: "user@example.com", Name: "Sneaky", Role: "consumer",
				SubscriptionType: "free", EventPreferences: []string{"Music"}}},
		{name: "email and id are read-only", who: consumer, id: consumer.sub,
			body: `{"id":"user-2","email":"hijack@example.com"}`, wantStatus: 200,
			want: &models.User{ID: consumer.sub, Email: "user@example.com", Name: "User", Role: "consumer",
				SubscriptionType: "free", EventPreferences: []string{"Music"}}},
		{name: "admin changes role", who: admin, id: consumer.sub, body: `{"role":"publisher"}`, wantStatus: 200,
			want: &models.User{ID: consumer.sub, Email: "user@example.com", Name: "User", Role: "publisher",
				SubscriptionType: "free", EventPreferences: []string{"Music"}}},
		{name: "admin replaces preferences", who: admin, id: consumer.sub, body: `{"eventPreferences":["Art","Food"]}`,
			wantStatus: 200, want: &models.User{ID: consumer.sub, Email: "user@example.com", Name: "User",
				Role: "consumer", SubscriptionType: "free", EventPreferences: []string{"Art", "Food"}}},
		{name: "admin invalid role", who: admin, id: consumer.sub, body: `{"role":"root"}`, wantStatus: 400},
		{name: "admin empty role", who: admin, id: consumer.sub, body: `{"role":""}`, wantStatus: 400},
		{name: "bad json", who: consumer, id: consumer.sub, body: `{"name":`, wantStatus: 400},
		{name: "not found", who: admin, id: "ghost", body: `{"name":"x"}`, wantStatus: 404},
		{name: "store get error", who: admin, id: consumer.sub, body: `{"name":"x"}`, storeErr: errDB, wantStatus: 500},
		{name: "store update error", who: admin, id: consumer.sub, body: `{"name":"x"}`, updateErr: errDB, wantStatus: 500},
		{name: "store update not found", who: admin, id: consumer.sub, body: `{"name":"x"}`,
			updateErr: store.ErrNotFound, wantStatus: 404},
		{name: "store update conflict", who: admin, id: consumer.sub, body: `{"name":"x"}`,
			updateErr: store.ErrConflict, wantStatus: 409},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeUserStore(sampleUsers()...)
			s.err = tt.storeErr
			s.updateErr = tt.updateErr
			rec := do(t, usersMux(s), "PUT", "/api/users/"+tt.id, tt.body, tt.who)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.want == nil {
				if tt.updateErr == nil && len(s.updated) != 0 {
					t.Fatalf("store must not be updated, got %+v", s.updated)
				}
				return
			}
			got := decode[models.User](t, rec)
			if !equalUser(&got, tt.want) {
				t.Fatalf("response = %+v, want %+v", got, *tt.want)
			}
			if !equalUser(s.users[tt.id], tt.want) {
				t.Fatalf("stored = %+v, want %+v", *s.users[tt.id], *tt.want)
			}
			if other := s.users["user-2"]; other.Email != "other@example.com" {
				t.Fatalf("other user modified: %+v", other)
			}
		})
	}
}

func equalUser(a, b *models.User) bool {
	return a.ID == b.ID && a.Email == b.Email && a.Name == b.Name && a.Role == b.Role &&
		a.SubscriptionType == b.SubscriptionType && a.PaymentMethod == b.PaymentMethod &&
		slices.Equal(a.Location, b.Location) && slices.Equal(a.EventPreferences, b.EventPreferences)
}

// Deleting users is Better Auth's job (the profile cascades), so the Go API
// exposes no DELETE route.
func TestUsersDeleteNotRouted(t *testing.T) {
	s := newFakeUserStore(sampleUsers()...)
	rec := do(t, usersMux(s), "DELETE", "/api/users/"+consumer.sub, "", admin)
	if rec.Code != 405 {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if _, ok := s.users[consumer.sub]; !ok {
		t.Fatal("user was deleted")
	}
}
