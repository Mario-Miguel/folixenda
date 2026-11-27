package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type SavedEventsHandler struct {
	store store.UserSavedEventStore
}

func NewSavedEventsHandler(s store.UserSavedEventStore) *SavedEventsHandler {
	return &SavedEventsHandler{store: s}
}

// Register mounts the saved-events routes. All act on the token's user.
func (h *SavedEventsHandler) Register(mux *http.ServeMux, protect Middleware) {
	mux.Handle("GET /api/savedEvents", chain(h.get, protect))
	mux.Handle("GET /api/savedEvents/{eventId}", chain(h.isSaved, protect))
	mux.Handle("POST /api/savedEvents", chain(h.create, protect))
	mux.Handle("DELETE /api/savedEvents/{eventId}", chain(h.delete, protect))
}

func (h *SavedEventsHandler) get(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
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

type isSavedResponse struct {
	Saved bool `json:"saved"`
}

// isSaved handles GET /api/savedEvents/{eventId} → {"saved": bool}.
func (h *SavedEventsHandler) isSaved(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	saved, err := h.store.IsSaved(claims.Subject, r.PathValue("eventId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check saved event")
		return
	}
	writeJSON(w, http.StatusOK, isSavedResponse{Saved: saved})
}

func (h *SavedEventsHandler) create(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}

	var userSavedEvent models.UserSavedEvent
	if err := json.NewDecoder(r.Body).Decode(&userSavedEvent); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if userSavedEvent.EventId == "" {
		writeError(w, http.StatusBadRequest, "eventId is required")
		return
	}
	if userSavedEvent.ID == "" {
		userSavedEvent.ID = uuid.New().String()
	}
	userSavedEvent.UserId = claims.Subject

	if err := h.store.Create(&userSavedEvent); err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "event already saved")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "event not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to save event")
		}
		return
	}
	writeJSON(w, http.StatusOK, userSavedEvent)
}

func (h *SavedEventsHandler) delete(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}

	if err := h.store.Delete(claims.Subject, r.PathValue("eventId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
