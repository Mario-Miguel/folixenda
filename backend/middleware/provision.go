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

func Provision(users UserUpserter) func(http.Handler) http.Handler {
	var seen sync.Map

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.UserFrom(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if _, done := seen.Load(claims.Subject); !done {
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

				seen.Store(claims.Subject, struct{}{})
			}

			next.ServeHTTP(w, r)
		})
	}
}
