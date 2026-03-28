package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-at-least-32-characters"

func newTestService() *Service {
	return &Service{jwtSecret: testSecret}
}

func TestGenerateTokenPair_ValidTokens(t *testing.T) {
	s := newTestService()
	pair, err := s.generateTokenPair("user-abc-123", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access token is empty")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token is empty")
	}
	if pair.ExpiresIn != 3600 {
		t.Errorf("expected ExpiresIn=3600, got %d", pair.ExpiresIn)
	}
}

func TestGenerateTokenPair_AccessTokenClaims(t *testing.T) {
	s := newTestService()
	pair, err := s.generateTokenPair("user-abc-123", "family")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	token, err := jwt.Parse(pair.AccessToken, func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse access token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to cast claims")
	}

	tests := []struct {
		key  string
		want string
	}{
		{"sub", "user-abc-123"},
		{"role", "family"},
		{"type", "access"},
	}
	for _, tt := range tests {
		got, _ := claims[tt.key].(string)
		if got != tt.want {
			t.Errorf("claim %q: expected %q, got %q", tt.key, tt.want, got)
		}
	}

	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("missing exp claim: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Error("access token is already expired")
	}
	if exp.After(time.Now().Add(2 * time.Hour)) {
		t.Error("access token expiry is too far in the future")
	}
}

func TestGenerateTokenPair_RefreshTokenClaims(t *testing.T) {
	s := newTestService()
	pair, err := s.generateTokenPair("user-abc-123", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	token, err := jwt.Parse(pair.RefreshToken, func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse refresh token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to cast claims")
	}

	typ, _ := claims["type"].(string)
	if typ != "refresh" {
		t.Errorf("expected type 'refresh', got %q", typ)
	}

	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("missing exp claim: %v", err)
	}
	if exp.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Error("refresh token expiry is too short")
	}
}

func TestRefreshToken_ValidRefresh(t *testing.T) {
	s := newTestService()
	original, err := s.generateTokenPair("user-xyz", "user")
	if err != nil {
		t.Fatalf("failed to generate initial tokens: %v", err)
	}

	newPair, err := s.RefreshToken(context.Background(), original.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if newPair.AccessToken == "" {
		t.Error("new access token is empty")
	}
	if newPair.RefreshToken == "" {
		t.Error("new refresh token is empty")
	}
	// Verify claims propagated correctly
	token, err := jwt.Parse(newPair.AccessToken, func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse new access token: %v", err)
	}
	claims := token.Claims.(jwt.MapClaims)
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if sub != "user-xyz" {
		t.Errorf("expected sub 'user-xyz', got %q", sub)
	}
	if role != "user" {
		t.Errorf("expected role 'user', got %q", role)
	}
}

func TestRefreshToken_RejectsAccessToken(t *testing.T) {
	s := newTestService()
	pair, err := s.generateTokenPair("user-xyz", "user")
	if err != nil {
		t.Fatalf("failed to generate tokens: %v", err)
	}

	_, err = s.RefreshToken(context.Background(), pair.AccessToken)
	if err == nil {
		t.Fatal("expected error when using access token as refresh token")
	}
	if err.Error() != "not a refresh token" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRefreshToken_RejectsExpiredToken(t *testing.T) {
	s := newTestService()

	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-xyz",
		"role": "user",
		"type": "refresh",
		"exp":  time.Now().Add(-time.Hour).Unix(),
	})
	tokenStr, err := expired.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = s.RefreshToken(context.Background(), tokenStr)
	if err == nil {
		t.Fatal("expected error for expired refresh token")
	}
	if err.Error() != "invalid refresh token" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRefreshToken_RejectsWrongSecret(t *testing.T) {
	wrongService := &Service{jwtSecret: "wrong-secret-at-least-32-chars!!"}

	s := newTestService()
	pair, err := s.generateTokenPair("user-xyz", "user")
	if err != nil {
		t.Fatalf("failed to generate tokens: %v", err)
	}

	_, err = wrongService.RefreshToken(
		context.Background(), pair.RefreshToken,
	)
	if err == nil {
		t.Fatal("expected error for token signed with different secret")
	}
	if err.Error() != "invalid refresh token" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRefreshToken_RejectsMalformedToken(t *testing.T) {
	s := newTestService()

	_, err := s.RefreshToken(context.Background(), "not-a-jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}
