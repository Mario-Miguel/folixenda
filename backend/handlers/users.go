package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type UsersHandler struct {
	store store.UserStore
}

func NewUsersHandler(s store.UserStore) *UsersHandler {
	return &UsersHandler{store: s}
}

func (h *UsersHandler) Register(mux *http.ServeMux, protect func(h http.Handler) http.Handler) {
	mux.Handle("GET /api/users", protect(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/users/{id}", protect(http.HandlerFunc(h.get)))
	mux.Handle("PUT /api/users/{id}", protect(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/users/{id}", protect(http.HandlerFunc(h.delete)))
}

func (h *UsersHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []*models.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *UsersHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *UsersHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var u models.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u.ID = id
	if err := h.store.Update(&u); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *UsersHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
