package auth

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	Email                string `json:"email"`
	Name                 string `json:"name"`
	Role                 string `json:"role"`
	jwt.RegisteredClaims        // sub, iss, aud, exp, iat
}
