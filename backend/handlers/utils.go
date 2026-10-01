package handlers

import (
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/auth"
)

func GetClaimsFromRequest(w http.ResponseWriter, r *http.Request) *auth.Claims {
	claims, ok := auth.UserFrom(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil
	}
	return claims

}
