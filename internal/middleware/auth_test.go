package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-at-least-32-characters"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-123",
		"role": "user",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))

	handler := middleware.Auth(secret, nil)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserIDFromContext(r.Context())
			if userID != "user-123" {
				t.Errorf("expected user-123, got %s", userID)
			}
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	handler := middleware.Auth("secret", nil)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	secret := "test-secret-at-least-32-characters"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))

	handler := middleware.Auth(secret, nil)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
