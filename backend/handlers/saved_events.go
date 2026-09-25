package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type SavedEventsHandler struct {
	store store.UserSavedEventStore
}

func NewSavedEventsHandler(s store.UserSavedEventStore) *SavedEventsHandler {
	return &SavedEventsHandler{store: s}
}

func (h *SavedEventsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/{userId}/savedEvents", h.get)
	mux.HandleFunc("POST /api/{userId}/savedEvents", h.create)
	mux.HandleFunc("DELETE /api/{userId}/savedEvents/{eventId}", h.delete)
}

func (h *SavedEventsHandler) get(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("userId")
	savedEvents, err := h.store.Get(userId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "saved events not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch saved events")
		return
	}
	writeJSON(w, http.StatusOK, savedEvents)
}

func (h *SavedEventsHandler) create(w http.ResponseWriter, r *http.Request) {
	var userSavedEvent models.UserSavedEvent
	userId := r.PathValue("userId")
	if err := json.NewDecoder(r.Body).Decode(&userSavedEvent); err != nil || userId != userSavedEvent.UserId {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.Create(&userSavedEvent); err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("could not create saved event: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, userSavedEvent)
}

func (h *SavedEventsHandler) delete(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("userId")
	eventId := r.PathValue("eventId")
	if err := h.store.Delete(userId, eventId); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
