package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/storage"
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

func claimsOf(who identity) *auth.Claims {
	return &auth.Claims{Role: who.role, RegisteredClaims: jwt.RegisteredClaims{Subject: who.sub}}
}

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
	// createErr, updateErr and deleteErr override err for that method only.
	createErr  error
	updateErr  error
	deleteErr  error
	lastFilter store.EventFilter
	created    []*models.Event
	// saves is the savesCount reported by ListByPublisher per event id.
	saves         map[string]int
	lastPublisher string
	deleted       []string
	getsLeft      int // when > 0, Get fails after this many more calls
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
		if filter.Category != "" && string(e.Category) != filter.Category {
			continue
		}
		// Like the real store: any session starting on Date / within [From, To].
		if slices.ContainsFunc(e.Sessions, func(s models.EventSession) bool {
			return (filter.Date == "" || s.Date == filter.Date) &&
				(filter.From == "" || s.Date >= filter.From) && (filter.To == "" || s.Date <= filter.To)
		}) {
			out = append(out, e)
		}
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
	if f.getsLeft > 0 {
		f.getsLeft--
		if f.getsLeft == 0 {
			return nil, errDB
		}
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
	e.Images = []models.EventImage{} // like the DB: a new event has no images
	e.ApplyImages()
	f.events[e.ID] = e
	f.created = append(f.created, e)
	return nil
}

func (f *fakeEventStore) Update(e *models.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.err != nil {
		return f.err
	}
	old, ok := f.events[e.ID]
	if !ok {
		return store.ErrNotFound
	}
	e.Images = old.Images // like the DB: images live in their own table
	e.ApplyImages()
	f.events[e.ID] = e
	return nil
}

func (f *fakeEventStore) Delete(id string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	if f.err != nil {
		return nil, f.err
	}
	e, ok := f.events[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	var keys []string
	for _, img := range e.Images {
		keys = append(keys, img.ObjectKey)
	}
	delete(f.events, id)
	f.deleted = append(f.deleted, id)
	return keys, nil
}

// failGetsAfter makes the n-th next Get fail (n > 0), to test the reload
// after a write.
func (f *fakeEventStore) failGetsAfter(n int) { f.getsLeft = n }

func (f *fakeEventStore) ListByPublisher(publisherID string) ([]*models.PublisherEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastPublisher = publisherID
	if f.err != nil {
		return nil, f.err
	}
	var out []*models.PublisherEvent
	for _, e := range f.events {
		if e.PublisherID == publisherID {
			out = append(out, &models.PublisherEvent{Event: *e, SavesCount: f.saves[e.ID]})
		}
	}
	slices.SortFunc(out, func(a, b *models.PublisherEvent) int {
		return strings.Compare(a.Date+a.StartTime, b.Date+b.StartTime)
	})
	return out, nil
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

// ---- publisher profiles ----

type fakeProfileStore struct {
	mu        sync.Mutex
	profiles  map[string]*models.PublisherProfile
	err       error // every method
	upsertErr error // Upsert only
	upserted  []models.PublisherProfile
}

func newFakeProfileStore(profiles ...*models.PublisherProfile) *fakeProfileStore {
	f := &fakeProfileStore{profiles: map[string]*models.PublisherProfile{}}
	for _, p := range profiles {
		f.profiles[p.UserID] = p
	}
	return f
}

func (f *fakeProfileStore) Get(userID string) (*models.PublisherProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	p, ok := f.profiles[userID]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakeProfileStore) Upsert(p *models.PublisherProfile) (*models.PublisherProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	if f.err != nil {
		return nil, f.err
	}
	cp := *p
	cp.CreatedAt, cp.UpdatedAt = "2026-11-27T10:00:00Z", "2026-11-27T11:00:00Z"
	if old, ok := f.profiles[p.UserID]; ok {
		cp.CreatedAt = old.CreatedAt
	}
	f.profiles[p.UserID] = &cp
	f.upserted = append(f.upserted, cp)
	out := cp
	return &out, nil
}

// ---- notification preferences ----

type fakePrefsStore struct {
	mu        sync.Mutex
	prefs     map[string]*models.NotificationPreferences
	err       error // every method
	upsertErr error // Upsert only
	upserted  []models.NotificationPreferences
}

func newFakePrefsStore() *fakePrefsStore {
	return &fakePrefsStore{prefs: map[string]*models.NotificationPreferences{}}
}

func (f *fakePrefsStore) Get(userID string) (*models.NotificationPreferences, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	p, ok := f.prefs[userID]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakePrefsStore) Upsert(userID string, p *models.NotificationPreferences) (*models.NotificationPreferences, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	if f.err != nil {
		return nil, f.err
	}
	cp := *p
	f.prefs[userID] = &cp
	f.upserted = append(f.upserted, cp)
	out := cp
	return &out, nil
}

// ---- event images + object storage ----

// fakeImageStore keeps images on the fakeEventStore's events, so Get sees them.
type fakeImageStore struct {
	events *fakeEventStore
	err    error
}

func (f *fakeImageStore) AddImage(img *models.EventImage, maxGallery int) (*models.EventImage, error) {
	f.events.mu.Lock()
	defer f.events.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	e, ok := f.events.events[img.EventID]
	if !ok {
		return nil, store.ErrNotFound
	}
	var replaced *models.EventImage
	if img.Kind == models.ImageKindCover {
		for i, old := range e.Images {
			if old.Kind == models.ImageKindCover {
				cp := old
				replaced = &cp
				e.Images = slices.Delete(e.Images, i, i+1)
				break
			}
		}
	} else {
		n := 0
		for _, old := range e.Images {
			if old.Kind == models.ImageKindGallery {
				n++
			}
		}
		if n >= maxGallery {
			return nil, store.ErrGalleryFull
		}
		img.Position = n
	}
	img.URL = "http://cdn/" + img.ObjectKey
	e.Images = append(e.Images, *img)
	e.ApplyImages()
	return replaced, nil
}

func (f *fakeImageStore) DeleteImage(eventID, imageID string) (*models.EventImage, error) {
	f.events.mu.Lock()
	defer f.events.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	e := f.events.events[eventID]
	for i, img := range e.Images {
		if img.ID == imageID {
			e.Images = slices.Delete(e.Images, i, i+1)
			e.ApplyImages()
			return &img, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeImageStore) ReorderImages(eventID string, ids []string) ([]models.EventImage, error) {
	f.events.mu.Lock()
	defer f.events.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	e := f.events.events[eventID]
	var gallery []string
	for _, img := range e.Images {
		if img.Kind == models.ImageKindGallery {
			gallery = append(gallery, img.ID)
		}
	}
	a, b := slices.Clone(gallery), slices.Clone(ids)
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		return nil, store.ErrInvalidInput
	}
	for i := range e.Images {
		if p := slices.Index(ids, e.Images[i].ID); p >= 0 {
			e.Images[i].Position = p
		}
	}
	return slices.Clone(e.Images), nil
}

// fakeObjects is an in-memory storage.Storage.
type fakeObjects struct {
	mu        sync.Mutex
	objects   map[string][]byte
	types     map[string]string
	putErr    error
	deleteErr error
	deleted   []string
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{objects: map[string][]byte{}, types: map[string]string{}}
}

func (f *fakeObjects) Put(_ context.Context, key string, r io.Reader, size int64, contentType string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil || int64(len(b)) != size {
		return errors.New("size mismatch")
	}
	f.objects[key], f.types[key] = b, contentType
	return nil
}

func (f *fakeObjects) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, key)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.objects, key)
	return nil
}

func (f *fakeObjects) URL(key string) string { return "http://cdn/" + key }

func (f *fakeObjects) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Sorted(maps.Keys(f.objects))
}

// Compile-time checks that the fakes satisfy the store interfaces.
var (
	_ store.EventStore                   = (*fakeEventStore)(nil)
	_ store.EventImageStore              = (*fakeImageStore)(nil)
	_ storage.Storage                    = (*fakeObjects)(nil)
	_ store.UserStore                    = (*fakeUserStore)(nil)
	_ store.UserSavedEventStore          = (*fakeSavedStore)(nil)
	_ store.PublisherProfileStore        = (*fakeProfileStore)(nil)
	_ store.NotificationPreferencesStore = (*fakePrefsStore)(nil)
)
