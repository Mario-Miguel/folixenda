package auth

import (
	"context"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type Verifier struct {
	keyfunc  jwt.Keyfunc
	issuer   string
	audience string
}

func NewVerifier(ctx context.Context, jwksURL, issuer, audience string) (*Verifier, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("loading JWKS from %s: %w", jwksURL, err)
	}

	verifier := &Verifier{keyfunc: k.Keyfunc, issuer: issuer, audience: audience}
	return verifier, nil
}

func (v *Verifier) Verify(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, v.keyfunc,
		jwt.WithValidMethods([]string{"EdDSA"}), // nunca aceptes "none" ni HS256
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	return claims, err
}
