package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/storage"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type EventsHandler struct {
	store   store.EventStore
	images  store.EventImageStore
	objects storage.Storage
}

// NewEventsHandler serves events and their images: rows in s and images,
// image files in objects.
func NewEventsHandler(s store.EventStore, images store.EventImageStore, objects storage.Storage) *EventsHandler {
	return &EventsHandler{store: s, images: images, objects: objects}
}

// Register mounts the events routes. Reads are public. Creating requires
// auth.CanPublish (the publish gate). Updating and deleting require an admin
// or publisher, and publishers may only touch their own events (see owns).
func (h *EventsHandler) Register(mux *http.ServeMux, protect Middleware) {
	canManage := middleware.RequireRole(auth.RoleAdmin, auth.RolePublisher)

	mux.HandleFunc("GET /api/events", h.list)
	mux.HandleFunc("GET /api/events/{id}", h.get)
	mux.Handle("POST /api/events", chain(h.create, protect, middleware.RequireCanPublish))
	mux.Handle("PUT /api/events/{id}", chain(h.update, protect, canManage))
	mux.Handle("DELETE /api/events/{id}", chain(h.delete, protect, canManage))

	mux.Handle("POST /api/events/{id}/images", chain(h.uploadImage, protect, canManage))
	mux.Handle("DELETE /api/events/{id}/images/{imageId}", chain(h.deleteImage, protect, canManage))
	mux.Handle("PUT /api/events/{id}/images/order", chain(h.reorderImages, protect, canManage))
}

// owns reports whether claims may modify e: admins any event, publishers only
// the events they own (never scraped ones, whose PublisherID is "").
func owns(claims *auth.Claims, e *models.Event) bool {
	if claims.IsAdmin() {
		return true
	}
	return claims.Role == auth.RolePublisher && claims.Subject != "" && e.PublisherID == claims.Subject
}

// loadOwned fetches the event at {id} and checks ownership, writing the error
// response (404, 403 or 500) and returning nil when the caller must stop.
func (h *EventsHandler) loadOwned(w http.ResponseWriter, r *http.Request) *models.Event {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return nil
	}
	existing, err := h.store.Get(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return nil
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch event")
		return nil
	}
	if !owns(claims, existing) {
		writeError(w, http.StatusForbidden, "you can only manage your own events")
		return nil
	}
	return existing
}

type listResponse struct {
	Events []*models.Event `json:"events"`
	Total  int             `json:"total"`
}

// list handles GET /api/events?date=&from=&to=&category= (dates YYYY-MM-DD,
// from/to inclusive, all optional and combinable).
func (h *EventsHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.EventFilter{
		Date:     q.Get("date"),
		From:     q.Get("from"),
		To:       q.Get("to"),
		Category: q.Get("category"),
	}
	if err := filter.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	events, err := h.store.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	if events == nil {
		events = []*models.Event{}
	}
	writeJSON(w, http.StatusOK, listResponse{Events: events, Total: len(events)})
}

func (h *EventsHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	event, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch event")
		return
	}
	writeJSON(w, http.StatusOK, event)
}

// Limits on event bodies (mirrored by the frontend's event editor).
const (
	maxSessions    = 60 // sessions (days) per event
	maxTitle       = 200
	maxDescription = 5000
	maxPerks       = 20
	maxPerk        = 80
)

// validClock reports whether v is a strict HH:MM 24-hour time.
func validClock(v string) bool {
	_, err := time.Parse("15:04", v)
	return err == nil && len(v) == 5
}

// prepareEvent validates a create/update body and normalizes it in place:
// sessions sorted and the first/last-session fields derived from them (any
// body date/startTime/endTime/endDate is ignored), images and organizer
// dropped. Returns the validation error message, or "".
func prepareEvent(e *models.Event) string {
	e.Title = strings.TrimSpace(e.Title)
	e.Venue = strings.TrimSpace(e.Venue)
	if e.Title == "" {
		return "title is required"
	}
	if utf8.RuneCountInString(e.Title) > maxTitle {
		return fmt.Sprintf("title must be at most %d characters", maxTitle)
	}
	if utf8.RuneCountInString(e.Description) > maxDescription {
		return fmt.Sprintf("description must be at most %d characters", maxDescription)
	}
	if e.Venue == "" {
		return "venue is required"
	}
	if e.Price < 0 {
		return "price must be 0 or more"
	}
	if len(e.Perks) > maxPerks {
		return fmt.Sprintf("perks must have at most %d entries", maxPerks)
	}
	for i, p := range e.Perks {
		if utf8.RuneCountInString(p) > maxPerk {
			return fmt.Sprintf("perks[%d] must be at most %d characters", i, maxPerk)
		}
	}
	if len(e.Sessions) == 0 || len(e.Sessions) > maxSessions {
		return fmt.Sprintf("sessions must have between 1 and %d entries", maxSessions)
	}
	seen := map[[2]string]bool{}
	for i, s := range e.Sessions {
		if _, err := time.Parse("2006-01-02", s.Date); err != nil || len(s.Date) != 10 {
			return fmt.Sprintf("sessions[%d].date must be YYYY-MM-DD", i)
		}
		if !validClock(s.StartTime) {
			return fmt.Sprintf("sessions[%d].startTime must be HH:MM", i)
		}
		if !validClock(s.EndTime) {
			return fmt.Sprintf("sessions[%d].endTime must be HH:MM", i)
		}
		k := [2]string{s.Date, s.StartTime}
		if seen[k] {
			return fmt.Sprintf("sessions[%d] duplicates another session (%s %s)", i, s.Date, s.StartTime)
		}
		seen[k] = true
	}
	e.Date, e.StartTime, e.EndTime, e.EndDate = "", "", "", ""
	e.ApplySessions()
	e.StoredImageURL = e.ImageURL
	e.Images = nil
	e.Organizer = nil
	return ""
}

// validImageURL reports whether an external imageUrl may be stored: empty or
// an absolute http(s) URL.
func validImageURL(v string) bool {
	if v == "" {
		return true
	}
	u, err := url.Parse(v)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

const badImageURL = "imageUrl must be empty or an absolute http(s) URL"

// reload re-reads the event after a write so the response carries what is
// stored (images, organizer). If that read fails the write still succeeded,
// so it falls back to the written event.
func (h *EventsHandler) reload(e *models.Event) *models.Event {
	fresh, err := h.store.Get(e.ID)
	if err != nil {
		slog.Warn("reload event after write", "id", e.ID, "err", err)
		e.ApplyImages()
		return e
	}
	return fresh
}

// create handles POST /api/events. Publishers always own what they create
// (any body publisherId is ignored); admins may set publisherId to create on
// a publisher's behalf, or omit it for an unowned event.
func (h *EventsHandler) create(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := prepareEvent(&event); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if !validImageURL(event.StoredImageURL) {
		writeError(w, http.StatusBadRequest, badImageURL)
		return
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if !claims.IsAdmin() {
		event.PublisherID = claims.Subject
	}
	if err := h.store.Create(&event); err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "event already exists")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusBadRequest, "publisherId does not match any user")
		default:
			writeError(w, http.StatusInternalServerError, "failed to create event")
		}
		return
	}
	writeJSON(w, http.StatusCreated, h.reload(&event))
}

// update handles PUT /api/events/{id}: a full replacement of the event's
// fields. The stored owner (publisherId) is always kept.
func (h *EventsHandler) update(w http.ResponseWriter, r *http.Request) {
	existing := h.loadOwned(w, r)
	if existing == nil {
		return
	}
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := prepareEvent(&event); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	event.ID = existing.ID
	event.PublisherID = existing.PublisherID
	// With an uploaded cover, imageUrl is derived from it: the body's value
	// (usually the round-tripped cover URL) is ignored and the stored external
	// image_url kept, for when the cover is deleted.
	if existing.Cover() != nil {
		event.StoredImageURL = existing.StoredImageURL
	} else if !validImageURL(event.StoredImageURL) {
		writeError(w, http.StatusBadRequest, badImageURL)
		return
	}
	if err := h.store.Update(&event); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update event")
		return
	}
	writeJSON(w, http.StatusOK, h.reload(&event))
}

// delete handles DELETE /api/events/{id}. The image objects are removed after
// the DB commit, best effort (a failure only leaves orphan objects).
func (h *EventsHandler) delete(w http.ResponseWriter, r *http.Request) {
	existing := h.loadOwned(w, r)
	if existing == nil {
		return
	}
	keys, err := h.store.Delete(existing.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	h.deleteObjects(r.Context(), keys...)
	w.WriteHeader(http.StatusNoContent)
}

// deleteObjects removes objects from storage, best effort: errors are only
// logged. It ignores the request's cancellation (the DB change is committed).
func (h *EventsHandler) deleteObjects(ctx context.Context, keys ...string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	for _, k := range keys {
		if err := h.objects.Delete(ctx, k); err != nil {
			slog.Error("delete image object", "key", k, "err", err)
		}
	}
}
