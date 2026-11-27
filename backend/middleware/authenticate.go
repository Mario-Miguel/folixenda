package middleware

import (
	"net/http"
	"strings"

	"github.com/Mario-Miguel/folixenda/backend/auth"
)

type TokenVerifier interface {
	Verify(string) (*auth.Claims, error)
}

// Authenticate verifies the "Authorization: Bearer <jwt>" header and stores the
// resulting claims in the request context. Missing or invalid tokens get a 401.
func Authenticate(v TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			token = strings.TrimSpace(token)
			if !ok || token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := v.Verify(token)
			if err != nil || claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), claims)))
		})
	}
}
