package jwt

import (
	"errors"
	gjwt "github.com/golang-jwt/jwt"
	"testing"
	"time"
)

func TestTokenValidation(t *testing.T) {
	key := []byte("test-key-that-is-at-least-32-bytes")
	tokens := NewJWT(key)
	claims := CustomClaims{Id: 123, StandardClaims: gjwt.StandardClaims{ExpiresAt: time.Now().Add(time.Hour).Unix()}}
	token, err := tokens.CreateToken(claims)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := tokens.ParseToken(token)
	if err != nil || parsed.Id != 123 {
		t.Fatalf("valid token: %#v %v", parsed, err)
	}
	if _, err := NewJWT([]byte("different-key")).ParseToken(token); err == nil {
		t.Fatal("wrong signing key accepted")
	}
	claims.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	expired, err := tokens.CreateToken(claims)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.ParseToken(expired); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired token: %v", err)
	}
	for _, invalid := range []string{"", "not-a-token"} {
		if _, err := tokens.ParseToken(invalid); err == nil {
			t.Fatal("malformed token accepted")
		}
	}
	claims.ExpiresAt = time.Now().Add(time.Hour).Unix()
	otherMethod, err := gjwt.NewWithClaims(gjwt.SigningMethodHS512, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.ParseToken(otherMethod); err == nil {
		t.Fatal("unexpected signing algorithm accepted")
	}
}
