package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer   = "http://localhost:3000"
	testAudience = "http://localhost:3000"
	testKID      = "test-key"
)

// newJWKSServer serves pub as an OKP/Ed25519 JWK, like Better Auth's
// /api/auth/jwks with its default EdDSA keys.
func newJWKSServer(t *testing.T, pub ed25519.PublicKey) *httptest.Server {
	t.Helper()
	body, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
		"kid": testKID,
		"alg": "EdDSA",
		"use": "sig",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func validClaims() *Claims {
	now := time.Now()
	return &Claims{
		Email: "ana@example.com",
		Name:  "Ana",
		Role:  RolePublisher,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "0b6f1c1e-0000-4000-8000-000000000001",
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, kid string, c *Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, c)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifier(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	srv := newJWKSServer(t, pub)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	v, err := NewVerifier(ctx, srv.URL, testIssuer, testAudience)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	mutate := func(f func(*Claims)) *Claims { c := validClaims(); f(c); return c }

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid", sign(t, jwt.SigningMethodEdDSA, priv, testKID, validClaims()), false},
		{"wrong issuer", sign(t, jwt.SigningMethodEdDSA, priv, testKID,
			mutate(func(c *Claims) { c.Issuer = "https://evil.example" })), true},
		{"missing issuer", sign(t, jwt.SigningMethodEdDSA, priv, testKID,
			mutate(func(c *Claims) { c.Issuer = "" })), true},
		{"wrong audience", sign(t, jwt.SigningMethodEdDSA, priv, testKID,
			mutate(func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} })), true},
		{"expired", sign(t, jwt.SigningMethodEdDSA, priv, testKID,
			mutate(func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) })), true},
		{"no expiry", sign(t, jwt.SigningMethodEdDSA, priv, testKID,
			mutate(func(c *Claims) { c.ExpiresAt = nil })), true},
		{"signed by another key", sign(t, jwt.SigningMethodEdDSA, otherPriv, testKID, validClaims()), true},
		{"unknown kid", sign(t, jwt.SigningMethodEdDSA, priv, "nope", validClaims()), true},
		{"RS256 not allowed", sign(t, jwt.SigningMethodRS256, rsaKey, testKID, validClaims()), true},
		{"HS256 not allowed", sign(t, jwt.SigningMethodHS256, []byte("secret"), testKID, validClaims()), true},
		{"alg none not allowed", sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, testKID, validClaims()), true},
		{"garbage", "not-a-jwt", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := v.Verify(tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			want := validClaims()
			if claims.Subject != want.Subject || claims.Email != want.Email ||
				claims.Name != want.Name || claims.Role != want.Role {
				t.Fatalf("claims = %+v, want %+v", claims, want)
			}
		})
	}
}

func TestNewVerifier_BadURL(t *testing.T) {
	if _, err := NewVerifier(context.Background(), "::not a url", testIssuer, testAudience); err == nil {
		t.Fatal("expected error for invalid JWKS URL")
	}
}

func TestContextRoundTrip(t *testing.T) {
	if _, ok := UserFrom(context.Background()); ok {
		t.Fatal("empty context should have no user")
	}
	c := validClaims()
	got, ok := UserFrom(WithUser(context.Background(), c))
	if !ok || got != c {
		t.Fatalf("UserFrom = %v, %v", got, ok)
	}
}

func TestRoles(t *testing.T) {
	for _, tt := range []struct {
		role  string
		valid bool
		admin bool
	}{
		{RoleAdmin, true, true},
		{RoleConsumer, true, false},
		{RolePublisher, true, false},
		{"", false, false},
		{"Admin", false, false},
		{"root", false, false},
	} {
		if got := ValidRole(tt.role); got != tt.valid {
			t.Errorf("ValidRole(%q) = %v", tt.role, got)
		}
		if got := (&Claims{Role: tt.role}).IsAdmin(); got != tt.admin {
			t.Errorf("IsAdmin(%q) = %v", tt.role, got)
		}
	}
	var nilClaims *Claims
	if nilClaims.IsAdmin() {
		t.Error("nil claims must not be admin")
	}
}
