package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

var errDB = errors.New("db exploded")

// Test identities, sent as headers and turned into claims by fakeProtect.
const (
	hdrSub  = "X-Test-Sub"
	hdrRole = "X-Test-Role"
)

// fakeProtect stands in for Authenticate+Provision: a request with an
// X-Test-Sub header gets claims {Subject, Role}; one without gets a 401.
func fakeProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub := r.Header.Get(hdrSub)
		if sub == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c := &auth.Claims{Role: r.Header.Get(hdrRole), RegisteredClaims: jwt.RegisteredClaims{Subject: sub}}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), c)))
	})
}

type identity struct{ sub, role string }

var (
	anon      = identity{}
	admin     = identity{"admin-1", auth.RoleAdmin}
	publisher = identity{"pub-1", auth.RolePublisher}
	consumer  = identity{"user-1", auth.RoleConsumer}
)

func do(t *testing.T, h http.Handler, method, target, body string, who identity) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if who.sub != "" {
		req.Header.Set(hdrSub, who.sub)
		req.Header.Set(hdrRole, who.role)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

// ---- events ----

type fakeEventStore struct {
	mu     sync.Mutex
	events map[string]*models.Event
	err    error // returned by every method when set
	// createErr overrides err for Create only.
	createErr  error
	lastFilter store.EventFilter
	created    []*models.Event
}

func newFakeEventStore(events ...*models.Event) *fakeEventStore {
	f := &fakeEventStore{events: map[string]*models.Event{}}
	for _, e := range events {
		f.events[e.ID] = e
	}
	return f
}

func (f *fakeEventStore) List(filter store.EventFilter) ([]*models.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	if f.err != nil {
		return nil, f.err
	}
	var out []*models.Event
	for _, e := range f.events {
		if filter.Date != "" && e.Date != filter.Date ||
			filter.From != "" && e.Date < filter.From ||
			filter.To != "" && e.Date > filter.To ||
			filter.Category != "" && string(e.Category) != filter.Category {
			continue
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b *models.Event) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (f *fakeEventStore) Get(id string) (*models.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	e, ok := f.events[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return e, nil
}

func (f *fakeEventStore) Create(e *models.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	if f.err != nil {
		return f.err
	}
	if _, ok := f.events[e.ID]; ok {
		return store.ErrConflict
	}
	f.events[e.ID] = e
	f.created = append(f.created, e)
	return nil
}

func (f *fakeEventStore) Update(e *models.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.events[e.ID]; !ok {
		return store.ErrNotFound
	}
	f.events[e.ID] = e
	return nil
}

func (f *fakeEventStore) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.events[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.events, id)
	return nil
}

// ---- users ----

type fakeUserStore struct {
	mu        sync.Mutex
	users     map[string]*models.User
	err       error // every method
	updateErr error // Update only
	updated   []models.User
}

func newFakeUserStore(users ...*models.User) *fakeUserStore {
	f := &fakeUserStore{users: map[string]*models.User{}}
	for _, u := range users {
		f.users[u.ID] = u
	}
	return f
}

func (f *fakeUserStore) List() ([]*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var out []*models.User
	for _, u := range f.users {
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b *models.User) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (f *fakeUserStore) Get(id string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUserStore) Update(u *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.err != nil {
		return f.err
	}
	if _, ok := f.users[u.ID]; !ok {
		return store.ErrNotFound
	}
	cp := *u
	f.users[u.ID] = &cp
	f.updated = append(f.updated, cp)
	return nil
}

func (f *fakeUserStore) Upsert(_ context.Context, u *models.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	cp := *u
	f.users[u.ID] = &cp
	return nil
}

// ---- saved events ----

type savedKey struct{ user, event string }

type fakeSavedStore struct {
	mu        sync.Mutex
	saved     map[savedKey]bool
	events    map[string]*models.Event // known events, for Create's FK check
	err       error
	createErr error
	created   []models.UserSavedEvent
}

func newFakeSavedStore(events ...*models.Event) *fakeSavedStore {
	f := &fakeSavedStore{saved: map[savedKey]bool{}, events: map[string]*models.Event{}}
	for _, e := range events {
		f.events[e.ID] = e
	}
	return f
}

func (f *fakeSavedStore) List() ([]*models.UserSavedEvents, error) {
	return nil, errors.New("not used by handlers")
}

func (f *fakeSavedStore) Get(userID string) (*models.UserSavedEvents, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := &models.UserSavedEvents{UserId: userID, SavedEvents: []models.SavedEvent{}}
	var ids []string
	for k := range f.saved {
		if k.user == userID {
			ids = append(ids, k.event)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		out.SavedEvents = append(out.SavedEvents, models.SavedEvent{Event: *f.events[id]})
	}
	return out, nil
}

func (f *fakeSavedStore) Create(se *models.UserSavedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	if f.err != nil {
		return f.err
	}
	if _, ok := f.events[se.EventId]; !ok {
		return store.ErrNotFound
	}
	k := savedKey{se.UserId, se.EventId}
	if f.saved[k] {
		return store.ErrConflict
	}
	f.saved[k] = true
	f.created = append(f.created, *se)
	return nil
}

func (f *fakeSavedStore) Delete(userID, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	k := savedKey{userID, eventID}
	if !f.saved[k] {
		return store.ErrNotFound
	}
	delete(f.saved, k)
	return nil
}

func (f *fakeSavedStore) DeleteById(string) error { return errors.New("not used by handlers") }

func (f *fakeSavedStore) IsSaved(userID, eventID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	return f.saved[savedKey{userID, eventID}], nil
}

// Compile-time checks that the fakes satisfy the store interfaces.
var (
	_ store.EventStore          = (*fakeEventStore)(nil)
	_ store.UserStore           = (*fakeUserStore)(nil)
	_ store.UserSavedEventStore = (*fakeSavedStore)(nil)
)
