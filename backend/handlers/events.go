package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type EventsHandler struct {
	store store.EventStore
}

func NewEventsHandler(s store.EventStore) *EventsHandler {
	return &EventsHandler{store: s}
}

// Register mounts the events routes. Reads are public; writes require an
// authenticated admin or publisher.
func (h *EventsHandler) Register(mux *http.ServeMux, protect Middleware) {
	canWrite := middleware.RequireRole(auth.RoleAdmin, auth.RolePublisher)

	mux.HandleFunc("GET /api/events", h.list)
	mux.HandleFunc("GET /api/events/{id}", h.get)
	mux.Handle("POST /api/events", chain(h.create, protect, canWrite))
	mux.Handle("PUT /api/events/{id}", chain(h.update, protect, canWrite))
	mux.Handle("DELETE /api/events/{id}", chain(h.delete, protect, canWrite))
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

// validateEvent checks the fields the store needs to build the timestamps.
func validateEvent(e *models.Event) string {
	if e.Title == "" {
		return "title is required"
	}
	if _, err := time.Parse("2006-01-02", e.Date); err != nil {
		return "date must be YYYY-MM-DD"
	}
	if _, err := time.Parse("15:04", e.StartTime); err != nil {
		return "startTime must be HH:MM"
	}
	if _, err := time.Parse("15:04", e.EndTime); err != nil {
		return "endTime must be HH:MM"
	}
	return ""
}

func (h *EventsHandler) create(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateEvent(&event); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if err := h.store.Create(&event); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "event already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create event")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (h *EventsHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateEvent(&event); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	event.ID = id
	if err := h.store.Update(&event); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update event")
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (h *EventsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
