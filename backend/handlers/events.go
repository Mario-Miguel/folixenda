package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type EventsHandler struct {
	store *store.Store
}

func NewEventsHandler(s *store.Store) *EventsHandler {
	return &EventsHandler{store: s}
}

// Register mounts all event routes onto mux.
func (h *EventsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events", h.list)
	mux.HandleFunc("GET /api/events/{id}", h.get)
	mux.HandleFunc("POST /api/events", h.create)
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
	if event.ID == "" || event.Title == "" {
		writeError(w, http.StatusBadRequest, "id and title are required")
		return
	}
	if err := h.store.Create(&event); err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("could not create event: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
