//go:build integration

package auth

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationSecret = "integration-test-secret-at-least-32-chars-long"

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
	googleID := "int-user-" + id
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

func TestRefreshToken_Integration_WithDatabaseSuccess(t *testing.T) {
	pool := testIntegrationPool(t)
	ctx := context.Background()
	s := NewService(pool, nil, integrationSecret)

	// 1. Insert test user with role 'user'
	userID := insertTestUser(t, ctx, pool, "user")
	defer cleanTestUser(t, ctx, pool, userID)

	// 2. Generate original tokens
	original, err := s.generateTokenPair(userID, "user")
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	// 3. Refresh should succeed
	refreshed, err := s.RefreshToken(ctx, original.RefreshToken)
	if err != nil {
		t.Fatalf("expected RefreshToken to succeed, got error: %v", err)
	}

	// Verify claims
	token, err := jwt.Parse(refreshed.AccessToken, func(t *jwt.Token) (any, error) {
		return []byte(integrationSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse refreshed access token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to cast claims")
	}
	if gotRole, _ := claims["role"].(string); gotRole != "user" {
		t.Errorf("expected role 'user', got %q", gotRole)
	}

	// 4. Update role in database to 'family'
	_, err = pool.Exec(ctx, "UPDATE users SET role = 'family' WHERE id = $1::uuid", userID)
	if err != nil {
		t.Fatalf("failed to update user role to family: %v", err)
	}

	// 5. Refresh again with the newly generated refresh token -> should reflect new role 'family'
	refreshed2, err := s.RefreshToken(ctx, refreshed.RefreshToken)
	if err != nil {
		t.Fatalf("expected second RefreshToken to succeed, got error: %v", err)
	}

	token2, err := jwt.Parse(refreshed2.AccessToken, func(t *jwt.Token) (any, error) {
		return []byte(integrationSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse second refreshed access token: %v", err)
	}
	claims2 := token2.Claims.(jwt.MapClaims)
	if gotRole, _ := claims2["role"].(string); gotRole != "family" {
		t.Errorf("expected updated role 'family', got %q", gotRole)
	}
}

func TestRefreshToken_Integration_UserMissing(t *testing.T) {
	pool := testIntegrationPool(t)
	ctx := context.Background()
	s := NewService(pool, nil, integrationSecret)

	// 1. Insert test user with role 'user'
	userID := insertTestUser(t, ctx, pool, "user")
	// Clean up immediately or in case of early return
	defer cleanTestUser(t, ctx, pool, userID)

	// 2. Generate original tokens
	original, err := s.generateTokenPair(userID, "user")
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	// 3. Delete the user from database
	cleanTestUser(t, ctx, pool, userID)

	// 4. Attempt to refresh using the old refresh token -> should fail
	_, err = s.RefreshToken(ctx, original.RefreshToken)
	if err == nil {
		t.Fatal("expected RefreshToken to fail when user is missing, but it succeeded")
	}

	if !strings.Contains(err.Error(), "no rows in result set") {
		t.Errorf("expected error to contain 'no rows in result set', got: %v", err)
	}
}
