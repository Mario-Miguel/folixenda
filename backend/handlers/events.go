package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type EventsHandler struct {
	store store.EventStore
}

func NewEventsHandler(s store.EventStore) *EventsHandler {
	return &EventsHandler{store: s}
}

func (h *EventsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events", h.list)
	mux.HandleFunc("GET /api/events/{id}", h.get)
	mux.HandleFunc("POST /api/events", h.create)
	mux.HandleFunc("PUT /api/events/{id}", h.update)
	mux.HandleFunc("DELETE /api/events/{id}", h.delete)
}

type listResponse struct {
	Events []*models.Event `json:"events"`
	Total  int             `json:"total"`
}

func (h *EventsHandler) list(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	category := r.URL.Query().Get("category")

	events, err := h.store.List(date, category)
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

func (h *EventsHandler) create(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if event.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if err := h.store.Create(&event); err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("could not create event: %v", err))
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
