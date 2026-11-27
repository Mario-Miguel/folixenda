package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/models"
)

// okHandler records whether it was reached and which claims it saw.
type okHandler struct {
	called bool
	claims *auth.Claims
}

func (h *okHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.claims, _ = auth.UserFrom(r.Context())
	w.WriteHeader(http.StatusTeapot)
}

func claimsFor(sub, role string) *auth.Claims {
	return &auth.Claims{
		Email:            sub + "@example.com",
		Name:             "User " + sub,
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub},
	}
}

func withClaims(r *http.Request, c *auth.Claims) *http.Request {
	if c == nil {
		return r
	}
	return r.WithContext(auth.WithUser(r.Context(), c))
}

// ---- Authenticate ----

type fakeVerifier struct {
	claims   *auth.Claims
	err      error
	gotToken string
	calls    int
}

func (f *fakeVerifier) Verify(token string) (*auth.Claims, error) {
	f.calls++
	f.gotToken = token
	return f.claims, f.err
}

func TestAuthenticate(t *testing.T) {
	valid := claimsFor("u1", auth.RoleConsumer)
	tests := []struct {
		name        string
		header      string
		verifier    *fakeVerifier
		wantStatus  int
		wantToken   string
		wantVerify  bool
		wantClaims  *auth.Claims
		wantReached bool
	}{
		{name: "missing header", header: "", verifier: &fakeVerifier{claims: valid}, wantStatus: 401},
		{name: "not bearer", header: "Basic abc", verifier: &fakeVerifier{claims: valid}, wantStatus: 401},
		{name: "empty bearer", header: "Bearer ", verifier: &fakeVerifier{claims: valid}, wantStatus: 401},
		{name: "invalid token", header: "Bearer bad", verifier: &fakeVerifier{err: errors.New("bad sig")},
			wantStatus: 401, wantToken: "bad", wantVerify: true},
		{name: "verifier returns nil claims", header: "Bearer tok", verifier: &fakeVerifier{},
			wantStatus: 401, wantToken: "tok", wantVerify: true},
		{name: "valid token", header: "Bearer good.jwt.token", verifier: &fakeVerifier{claims: valid},
			wantStatus: http.StatusTeapot, wantToken: "good.jwt.token", wantVerify: true,
			wantClaims: valid, wantReached: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &okHandler{}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			Authenticate(tt.verifier)(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if next.called != tt.wantReached {
				t.Fatalf("next called = %v, want %v", next.called, tt.wantReached)
			}
			if (tt.verifier.calls > 0) != tt.wantVerify {
				t.Fatalf("verify called = %d times, want called=%v", tt.verifier.calls, tt.wantVerify)
			}
			if tt.wantVerify && tt.verifier.gotToken != tt.wantToken {
				t.Fatalf("token = %q, want %q", tt.verifier.gotToken, tt.wantToken)
			}
			if next.claims != tt.wantClaims {
				t.Fatalf("claims in context = %v, want %v", next.claims, tt.wantClaims)
			}
		})
	}
}

// ---- RequireRole / RequireSelfOrRole ----

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		roles      []string
		wantStatus int
	}{
		{"no claims", nil, []string{auth.RoleAdmin}, 401},
		{"admin allowed", claimsFor("a", auth.RoleAdmin), []string{auth.RoleAdmin}, http.StatusTeapot},
		{"publisher allowed among several", claimsFor("p", auth.RolePublisher),
			[]string{auth.RoleAdmin, auth.RolePublisher}, http.StatusTeapot},
		{"consumer forbidden", claimsFor("c", auth.RoleConsumer),
			[]string{auth.RoleAdmin, auth.RolePublisher}, 403},
		{"empty role forbidden", claimsFor("x", ""), []string{auth.RoleAdmin}, 403},
		{"no roles allows nobody", claimsFor("a", auth.RoleAdmin), nil, 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &okHandler{}
			req := withClaims(httptest.NewRequest(http.MethodGet, "/", nil), tt.claims)
			rec := httptest.NewRecorder()
			RequireRole(tt.roles...)(next).ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if next.called != (tt.wantStatus == http.StatusTeapot) {
				t.Fatalf("next called = %v", next.called)
			}
		})
	}
}

func TestRequireSelfOrRole(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		pathID     string
		wantStatus int
	}{
		{"no claims", nil, "u1", 401},
		{"self", claimsFor("u1", auth.RoleConsumer), "u1", http.StatusTeapot},
		{"other user forbidden", claimsFor("u1", auth.RoleConsumer), "u2", 403},
		{"publisher on other forbidden", claimsFor("u1", auth.RolePublisher), "u2", 403},
		{"admin on other allowed", claimsFor("a", auth.RoleAdmin), "u2", http.StatusTeapot},
		{"empty subject never matches", claimsFor("", auth.RoleConsumer), "", 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &okHandler{}
			mux := http.NewServeMux()
			mux.Handle("GET /users/{id}", RequireSelfOrRole("id", auth.RoleAdmin)(next))
			mux.Handle("GET /users/", RequireSelfOrRole("id", auth.RoleAdmin)(next)) // empty id
			req := withClaims(httptest.NewRequest(http.MethodGet, "/users/"+tt.pathID, nil), tt.claims)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// ---- Provision ----

type fakeUpserter struct {
	mu    sync.Mutex
	calls []models.User
	err   error
}

func (f *fakeUpserter) Upsert(_ context.Context, u *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, *u)
	return f.err
}

func TestProvision_NoClaims(t *testing.T) {
	up := &fakeUpserter{}
	next := &okHandler{}
	rec := httptest.NewRecorder()
	Provision(up)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 401 || next.called || len(up.calls) != 0 {
		t.Fatalf("status=%d called=%v upserts=%d", rec.Code, next.called, len(up.calls))
	}
}

func TestProvision_UpsertError(t *testing.T) {
	up := &fakeUpserter{err: errors.New("db down")}
	next := &okHandler{}
	h := Provision(up)(next)
	for i := range 2 { // a failed upsert must not be cached
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withClaims(httptest.NewRequest(http.MethodGet, "/", nil), claimsFor("u1", "consumer")))
		if rec.Code != 500 || next.called {
			t.Fatalf("request %d: status=%d called=%v", i, rec.Code, next.called)
		}
	}
	if len(up.calls) != 2 {
		t.Fatalf("upserts = %d, want 2 (failure not cached)", len(up.calls))
	}
}

func TestProvision_Sequence(t *testing.T) {
	steps := []struct {
		name       string
		claims     *auth.Claims
		wantUpsert bool
	}{
		{"first request upserts", claimsFor("u1", auth.RoleConsumer), true},
		{"same claims cached", claimsFor("u1", auth.RoleConsumer), false},
		{"other user upserts", claimsFor("u2", auth.RoleConsumer), true},
		{"role change re-upserts", claimsFor("u1", auth.RolePublisher), true},
		{"new role now cached", claimsFor("u1", auth.RolePublisher), false},
		{"name change re-upserts", func() *auth.Claims {
			c := claimsFor("u1", auth.RolePublisher)
			c.Name = "Renamed"
			return c
		}(), true},
		{"email change re-upserts", func() *auth.Claims {
			c := claimsFor("u1", auth.RolePublisher)
			c.Name = "Renamed"
			c.Email = "new@example.com"
			return c
		}(), true},
	}
	up := &fakeUpserter{}
	h := Provision(up)(&okHandler{})
	for _, s := range steps {
		before := len(up.calls)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withClaims(httptest.NewRequest(http.MethodGet, "/", nil), s.claims))
		if rec.Code != http.StatusTeapot {
			t.Fatalf("%s: status = %d", s.name, rec.Code)
		}
		upserted := len(up.calls) > before
		if upserted != s.wantUpsert {
			t.Fatalf("%s: upserted = %v, want %v", s.name, upserted, s.wantUpsert)
		}
		if upserted {
			got := up.calls[len(up.calls)-1]
			want := models.User{ID: s.claims.Subject, Email: s.claims.Email, Name: s.claims.Name, Role: s.claims.Role}
			if got.ID != want.ID || got.Email != want.Email || got.Name != want.Name || got.Role != want.Role {
				t.Fatalf("%s: upserted %+v, want %+v", s.name, got, want)
			}
		}
	}
}

func TestProvision_Concurrent(t *testing.T) {
	up := &fakeUpserter{}
	h := Provision(up)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withClaims(httptest.NewRequest(http.MethodGet, "/", nil), claimsFor("u1", "consumer")))
		})
	}
	wg.Wait()
	if n := len(up.calls); n < 1 || n > 20 {
		t.Fatalf("upserts = %d", n)
	}
}

// ---- Cors ----

func TestCors(t *testing.T) {
	tests := []struct {
		method      string
		wantStatus  int
		wantReached bool
	}{
		{http.MethodOptions, http.StatusNoContent, false},
		{http.MethodGet, http.StatusTeapot, true},
		{http.MethodPost, http.StatusTeapot, true},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			next := &okHandler{}
			rec := httptest.NewRecorder()
			Cors(next).ServeHTTP(rec, httptest.NewRequest(tt.method, "/api/events", nil))
			if rec.Code != tt.wantStatus || next.called != tt.wantReached {
				t.Fatalf("status=%d called=%v", rec.Code, next.called)
			}
			h := rec.Header()
			if h.Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("allow-origin = %q", h.Get("Access-Control-Allow-Origin"))
			}
			for _, m := range []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"} {
				if !strings.Contains(h.Get("Access-Control-Allow-Methods"), m) {
					t.Errorf("allow-methods %q missing %s", h.Get("Access-Control-Allow-Methods"), m)
				}
			}
			if !strings.Contains(h.Get("Access-Control-Allow-Headers"), "Authorization") {
				t.Errorf("allow-headers = %q", h.Get("Access-Control-Allow-Headers"))
			}
		})
	}
}

// ---- Logging ----

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	next := &okHandler{}
	rec := httptest.NewRecorder()
	Logging(logger, next).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/events/42", nil))
	if !next.called || rec.Code != http.StatusTeapot {
		t.Fatalf("next not called / status %d", rec.Code)
	}
	out := buf.String()
	for _, want := range []string{"msg=request", "method=DELETE", "path=/api/events/42"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}
