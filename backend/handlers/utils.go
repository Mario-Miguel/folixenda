package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Mario-Miguel/folixenda/backend/auth"
)

// Middleware wraps a handler. Register methods receive "protect", the
// authentication chain (Authenticate → Provision), and layer role checks from
// the middleware package on top of it.
type Middleware = func(http.Handler) http.Handler

// chain applies mws so that the first one is the outermost.
func chain(h http.HandlerFunc, mws ...Middleware) http.Handler {
	var out http.Handler = h
	for i := len(mws) - 1; i >= 0; i-- {
		out = mws[i](out)
	}
	return out
}

func GetClaimsFromRequest(w http.ResponseWriter, r *http.Request) *auth.Claims {
	claims, ok := auth.UserFrom(r.Context())
	if !ok || claims == nil || claims.Subject == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil
	}
	return claims
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
