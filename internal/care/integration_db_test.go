//go:build integration

package care

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestIntegration_HasFamilyLinkBetween(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	store := NewStore(pool)

	cared := uuid.NewString()
	family := uuid.NewString()
	g1 := "int-care-" + cared
	g2 := "int-fam-" + family

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, google_id, email, display_name, role) VALUES
			($1::uuid, $2, 'int_cared@test', 'Int Cared', 'user'),
			($3::uuid, $4, 'int_family@test', 'Int Family', 'family')
	`, cared, g1, family, g2)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM family_links WHERE user_id::text IN ($1,$2) OR family_id::text IN ($1,$2)`, cared, family)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id::text IN ($1,$2)`, cared, family)
	})

	ok, err := store.HasFamilyLinkBetween(ctx, cared, family)
	if err != nil {
		t.Fatalf("HasFamilyLinkBetween: %v", err)
	}
	if ok {
		t.Fatal("expected no link before insert")
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO family_links (user_id, family_id, relationship, is_primary_contact)
		VALUES ($1::uuid, $2::uuid, 'caregiver', true)
	`, family, cared)
	if err != nil {
		t.Fatalf("insert family_links: %v", err)
	}

	ok, err = store.HasFamilyLinkBetween(ctx, cared, family)
	if err != nil {
		t.Fatalf("HasFamilyLinkBetween: %v", err)
	}
	if !ok {
		t.Fatal("expected link after insert")
	}
}

func TestIntegration_GetCareAlertByIDAndCaredLookup(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	store := NewStore(pool)

	userID := uuid.NewString()
	g := "int-alert-user-" + userID
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, google_id, email, display_name, role)
		VALUES ($1::uuid, $2, 'int_alert@test', 'Int Alert User', 'user')
	`, userID, g)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM care_alerts WHERE user_id::text = $1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id::text = $1`, userID)
	})

	var alertID string
	err = pool.QueryRow(ctx, `
		INSERT INTO care_alerts (user_id, rule_code, severity, title, status)
		VALUES ($1::uuid, 'test_rule', 'info', 't', 'open')
		RETURNING id::text
	`, userID).Scan(&alertID)
	if err != nil {
		t.Fatalf("insert alert: %v", err)
	}

	row, err := store.GetCareAlertByID(ctx, alertID)
	if err != nil {
		t.Fatalf("GetCareAlertByID: %v", err)
	}
	if row == nil || row.UserID != userID {
		t.Fatalf("unexpected row: %+v", row)
	}

	svc := NewService(store, nil)
	cared, err := svc.CaredUserIDForCareAlert(ctx, alertID)
	if err != nil {
		t.Fatalf("CaredUserIDForCareAlert: %v", err)
	}
	if cared != userID {
		t.Fatalf("cared = %q want %q", cared, userID)
	}
}
