package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
	"github.com/google/uuid"
)

type SavedEventsHandler struct {
	store store.UserSavedEventStore
}

func NewSavedEventsHandler(s store.UserSavedEventStore) *SavedEventsHandler {
	return &SavedEventsHandler{store: s}
}

func (h *SavedEventsHandler) Register(mux *http.ServeMux, protect func(h http.Handler) http.Handler) {
	mux.Handle("GET /api/savedEvents", protect(http.HandlerFunc(h.get)))
	mux.Handle("POST /api/savedEvents", protect(http.HandlerFunc(h.create)))
	mux.Handle("DELETE /api/savedEvents/{eventId}", protect(http.HandlerFunc(h.delete)))
}

func (h *SavedEventsHandler) get(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)

	if claims == nil || claims.Subject == "" {
		return
	}

	savedEvents, err := h.store.Get(claims.Subject)
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
	claims := GetClaimsFromRequest(w, r)

	if claims == nil || claims.Subject == "" {
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&userSavedEvent); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if userSavedEvent.ID == "" {
		userSavedEvent.ID = uuid.New().String()
	}

	userSavedEvent.UserId = claims.Subject

	if err := h.store.Create(&userSavedEvent); err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("could not create saved event: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, userSavedEvent)
}

func (h *SavedEventsHandler) delete(w http.ResponseWriter, r *http.Request) {
	eventId := r.PathValue("eventId")
	claims := GetClaimsFromRequest(w, r)

	if claims == nil || claims.Subject == "" {
		return
	}

	if err := h.store.Delete(claims.Subject, eventId); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
