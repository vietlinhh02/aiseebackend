package middleware_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

func TestAuthMiddleware_WrongSigningMethod(t *testing.T) {
	secret := "test-secret-at-least-32-characters"

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub":  "user-123",
		"role": "user",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, err := token.SignedString(ecKey)
	if err != nil {
		t.Fatal(err)
	}

	handler := middleware.Auth(secret)(http.HandlerFunc(
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

func TestAuthMiddleware_EmptyBearerToken(t *testing.T) {
	handler := middleware.Auth("secret")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-123",
		"role": "user",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte("secret-one-32-chars-long-xxxxx"))

	handler := middleware.Auth("secret-two-32-chars-long-xxxxx")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_RoleInContext(t *testing.T) {
	secret := "test-secret-at-least-32-characters"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-456",
		"role": "family",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(secret))

	handler := middleware.Auth(secret)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			role := middleware.RoleFromContext(r.Context())
			if role != "family" {
				t.Errorf("expected role 'family', got %q", role)
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

func TestAuthMiddleware_MalformedToken(t *testing.T) {
	handler := middleware.Auth("secret")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt-at-all")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}

	var body response.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "invalid_token" {
		t.Errorf("expected error code 'invalid_token', got %q", body.Error.Code)
	}
}

func TestInternalAuth_MissingHeader(t *testing.T) {
	handler := middleware.InternalAuth("test-key")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestInternalAuth_EmptyKey(t *testing.T) {
	handler := middleware.InternalAuth("test-key")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Internal-Key", "")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
