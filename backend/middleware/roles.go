package middleware

import (
	"net/http"
	"slices"

	"github.com/Mario-Miguel/folixenda/backend/auth"
)

// RequireRole only lets through requests whose claims carry one of roles.
// It must run after Authenticate: no claims → 401, role not allowed → 403.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.UserFrom(r.Context())
			if !ok || claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !slices.Contains(roles, claims.Role) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSelfOrRole lets through requests where the path value named param
// equals the token subject (the user acting on themself), or whose role is one
// of roles. No claims → 401, otherwise → 403.
func RequireSelfOrRole(param string, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.UserFrom(r.Context())
			if !ok || claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			self := claims.Subject != "" && claims.Subject == r.PathValue(param)
			if !self && !slices.Contains(roles, claims.Role) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireCanPublish only lets through requests whose claims may publish events,
// as decided by auth.CanPublish. No claims → 401, not allowed → 403.
func RequireCanPublish(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.UserFrom(r.Context())
		if !ok || claims == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !auth.CanPublish(claims) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
