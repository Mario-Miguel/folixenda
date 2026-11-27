package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/Mario-Miguel/folixenda/backend/auth"
	"github.com/Mario-Miguel/folixenda/backend/models"
)

type UserUpserter interface {
	Upsert(ctx context.Context, u *models.User) error
}

// profile is the part of the claims mirrored into the users table.
type profile struct {
	email, name, role string
}

// Provision upserts the authenticated user's profile (id, email, name, role)
// into the users table. It caches the last profile written per subject and
// upserts again whenever the token's email, name or role differ from it, so a
// role change in Better Auth reaches Go as soon as a fresh token is used.
func Provision(users UserUpserter) func(http.Handler) http.Handler {
	var seen sync.Map // subject → profile

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.UserFrom(r.Context())
			if !ok || claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			current := profile{email: claims.Email, name: claims.Name, role: claims.Role}
			if cached, ok := seen.Load(claims.Subject); !ok || cached.(profile) != current {
				u := &models.User{
					ID:    claims.Subject,
					Email: claims.Email,
					Name:  claims.Name,
					Role:  claims.Role,
				}
				if err := users.Upsert(r.Context(), u); err != nil {
					http.Error(w, "could not provision user", http.StatusInternalServerError)
					return
				}
				seen.Store(claims.Subject, current)
			}

			next.ServeHTTP(w, r)
		})
	}
}
