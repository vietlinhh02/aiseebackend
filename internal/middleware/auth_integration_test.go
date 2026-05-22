//go:build integration

package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/middleware"
)

const integrationSecret = "middleware-integration-test-secret-32-chars-long"

func testIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect to pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func insertTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, role string) string {
	t.Helper()
	id := uuid.NewString()
	googleID := "middleware-int-user-" + id
	email := id + "@test.local"
	name := "Integration " + role + " " + id[:8]
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, google_id, email, display_name, role)
		VALUES ($1::uuid, $2, $3, $4, $5)
	`, id, googleID, email, name, role); err != nil {
		t.Fatalf("failed to insert user %s: %v", role, err)
	}
	return id
}

func cleanTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) {
	t.Helper()
	_, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1::uuid", id)
	if err != nil {
		t.Logf("cleanup user warning %s: %v", id, err)
	}
}

func TestAuthMiddleware_Integration_DatabaseFallbackSuccess(t *testing.T) {
	pool := testIntegrationPool(t)
	ctx := context.Background()

	// 1. Insert test user with role 'user'
	userID := insertTestUser(t, ctx, pool, "user")
	defer cleanTestUser(t, ctx, pool, userID)

	// 2. Generate token with role 'user'
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": "user",
		"type": "access",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(integrationSecret))

	// 3. Construct Auth middleware with database pool (and no Redis)
	handler := middleware.Auth(integrationSecret, nil, pool)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotUserID := middleware.UserIDFromContext(r.Context())
			if gotUserID != userID {
				t.Errorf("expected user ID %q, got %q", userID, gotUserID)
			}
			w.WriteHeader(http.StatusOK)
		},
	))

	// 4. Request should succeed (allowed via DB check)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_Integration_DatabaseFallbackUserMissing(t *testing.T) {
	pool := testIntegrationPool(t)
	ctx := context.Background()

	// 1. Insert test user with role 'user'
	userID := insertTestUser(t, ctx, pool, "user")
	defer cleanTestUser(t, ctx, pool, userID)

	// 2. Generate token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": "user",
		"type": "access",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(integrationSecret))

	// 3. Construct Auth middleware
	handler := middleware.Auth(integrationSecret, nil, pool)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	// 4. Delete the user from the database
	cleanTestUser(t, ctx, pool, userID)

	// 5. Request should fail (rejected via DB check)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected HTTP 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "invalid_user") {
		t.Errorf("expected body to contain 'invalid_user', got %s", rr.Body.String())
	}
}

func TestAuthMiddleware_Integration_DatabaseFallbackRoleMismatch(t *testing.T) {
	pool := testIntegrationPool(t)
	ctx := context.Background()

	// 1. Insert test user with role 'user'
	userID := insertTestUser(t, ctx, pool, "user")
	defer cleanTestUser(t, ctx, pool, userID)

	// 2. Generate token but with role 'family' (mismatch)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": "family",
		"type": "access",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(integrationSecret))

	// 3. Construct Auth middleware
	handler := middleware.Auth(integrationSecret, nil, pool)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	// 4. Request should fail (rejected via role mismatch)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected HTTP 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "role_changed") {
		t.Errorf("expected body to contain 'role_changed', got %s", rr.Body.String())
	}
}
