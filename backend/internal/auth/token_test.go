package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestTokenIssuerRoundTrip(t *testing.T) {
	issuer := NewTokenIssuer(testSecret, "crispy-a", time.Minute)

	token, expiresAt, err := issuer.Issue(42, "admin@example.com")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatalf("expected future expiry, got %s", expiresAt)
	}

	identity, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if identity.ID != 42 {
		t.Fatalf("expected user id 42, got %d", identity.ID)
	}
	if identity.Email != "admin@example.com" {
		t.Fatalf("expected email admin@example.com, got %q", identity.Email)
	}
}

func TestTokenIssuerRejectsExpiredToken(t *testing.T) {
	issuer := NewTokenIssuer(testSecret, "crispy-a", -time.Minute)

	token, _, err := issuer.Issue(7, "admin@example.com")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if _, err := issuer.Parse(token); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestTokenIssuerRejectsForeignIssuer(t *testing.T) {
	issuing := NewTokenIssuer(testSecret, "issuer-a", time.Minute)
	verifying := NewTokenIssuer(testSecret, "issuer-b", time.Minute)

	token, _, err := issuing.Issue(1, "admin@example.com")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if _, err := verifying.Parse(token); err == nil {
		t.Fatal("token signed for another issuer must be rejected")
	}
}

func TestTokenIssuerRejectsUnsignedToken(t *testing.T) {
	issuer := NewTokenIssuer(testSecret, "crispy-a", time.Minute)
	claims := Claims{
		Email: "admin@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			Issuer:    "crispy-a",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}

	raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build unsigned token: %v", err)
	}

	if _, err := issuer.Parse(raw); err == nil {
		t.Fatal("unsigned token must be rejected")
	}
}
