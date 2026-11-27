package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/middleware"
	"github.com/Mario-Miguel/folixenda/backend/models"
	"github.com/Mario-Miguel/folixenda/backend/store"
)

type UsersHandler struct {
	store store.UserStore
}

func NewUsersHandler(s store.UserStore) *UsersHandler {
	return &UsersHandler{store: s}
}

// Register mounts the users routes, all authenticated:
//   - list:         admin only
//   - get / update: admin, or the user themself
//
// There is no DELETE: users are deleted in Better Auth (frontend admin panel)
// and the profile goes with them via ON DELETE CASCADE on users.id.
func (h *UsersHandler) Register(mux *http.ServeMux, protect Middleware) {
	adminOnly := middleware.RequireRole(auth.RoleAdmin)
	selfOrAdmin := middleware.RequireSelfOrRole("id", auth.RoleAdmin)

	mux.Handle("GET /api/users", chain(h.list, protect, adminOnly))
	mux.Handle("GET /api/users/{id}", chain(h.get, protect, selfOrAdmin))
	mux.Handle("PUT /api/users/{id}", chain(h.update, protect, selfOrAdmin))
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

// update applies a partial update: fields present in the JSON body overwrite
// the stored profile, absent ones are kept. id and email are owned by Better
// Auth and always kept. Only admins can change the role; for anyone else the
// "role" field is ignored. The store mirrors name/role into auth."user".
func (h *UsersHandler) update(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromRequest(w, r)
	if claims == nil {
		return
	}
	id := r.PathValue("id")
	existing, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	u := *existing
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u.ID = existing.ID
	u.Email = existing.Email
	if !claims.IsAdmin() {
		u.Role = existing.Role
	} else if !auth.ValidRole(u.Role) {
		writeError(w, http.StatusBadRequest, "role must be one of admin, consumer, publisher")
		return
	} else if !auth.AccountTypeChangeAllowed(existing.Role, u.Role) {
		writeError(w, http.StatusConflict, "account type can't change between consumer and publisher: create a new account")
		return
	}

	if err := h.store.Update(&u); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "user not found")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "conflicting user data")
		default:
			writeError(w, http.StatusInternalServerError, "failed to update user")
		}
		return
	}
	writeJSON(w, http.StatusOK, u)
}
