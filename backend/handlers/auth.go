package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/store"
)

type AuthHandler struct {
	users store.UserStore
}

func NewAuthHandler(users store.UserStore) *AuthHandler {
	return &AuthHandler{users: users}
}

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.login)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.users.Authenticate(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "authentication failed")
		return
	}
	writeJSON(w, http.StatusOK, user)
}
