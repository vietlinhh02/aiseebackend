//go:build integration

package user

import (
	"context"
	"errors"
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

func insertIntegrationUser(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	role string,
) string {
	t.Helper()
	id := uuid.NewString()
	googleID := "int-user-" + id
	email := id + "@test.local"
	name := "Int " + role + " " + id[:8]
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, google_id, email, display_name, role)
		VALUES ($1::uuid, $2, $3, $4, $5)
	`, id, googleID, email, name, role); err != nil {
		t.Fatalf("insert user %s: %v", role, err)
	}
	return id
}

func cleanupIntegrationUsers(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	ids ...string,
) {
	t.Helper()
	for _, id := range ids {
		if id == "" {
			continue
		}
		_, _ = pool.Exec(ctx, `DELETE FROM family_invites WHERE used_by_user_id::text = $1 OR created_by_user_id::text = $1 OR owner_user_id::text = $1 OR cared_user_id::text = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM family_links WHERE user_id::text = $1 OR family_id::text = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM care_group_cared_users WHERE user_id::text = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM care_group_members WHERE user_id::text = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM care_groups WHERE owner_user_id::text = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id::text = $1`, id)
	}
}

func TestIntegration_CreateInviteAllowsFamilyOwner(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerID := insertIntegrationUser(t, ctx, pool, "family")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerID)
	})

	invite, err := svc.CreateInvite(ctx, ownerID, "Gia dinh")
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	if len(invite.Code) != 6 {
		t.Fatalf("invite code length = %d, want 6", len(invite.Code))
	}

	var careGroupID, storedOwnerID, createdByUserID string
	var maxUses int
	err = pool.QueryRow(ctx, `
		SELECT care_group_id::text, owner_user_id::text, created_by_user_id::text, max_uses
		FROM family_invites
		WHERE code = $1
	`, invite.Code).Scan(&careGroupID, &storedOwnerID, &createdByUserID, &maxUses)
	if err != nil {
		t.Fatalf("load family_invites row: %v", err)
	}
	if careGroupID == "" {
		t.Fatal("expected invite to reference a care group")
	}
	if storedOwnerID != ownerID {
		t.Fatalf("owner_user_id = %q, want %q", storedOwnerID, ownerID)
	}
	if createdByUserID != ownerID {
		t.Fatalf("created_by_user_id = %q, want %q", createdByUserID, ownerID)
	}
	if maxUses != 1 {
		t.Fatalf("max_uses = %d, want 1", maxUses)
	}
}

func TestIntegration_JoinFamilyAddsUserAsCaredUserAndMarksInviteUsed(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerID := insertIntegrationUser(t, ctx, pool, "family")
	patientID := insertIntegrationUser(t, ctx, pool, "user")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerID, patientID)
	})

	invite, err := svc.CreateInvite(ctx, ownerID, "Gia dinh")
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}

	if err := svc.JoinFamily(ctx, patientID, invite.Code); err != nil {
		t.Fatalf("JoinFamily() error = %v", err)
	}

	var caredCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM care_group_cared_users cgu
		JOIN family_invites fi ON fi.care_group_id = cgu.care_group_id
		WHERE fi.code = $1
		  AND cgu.user_id = $2::uuid
		  AND cgu.deleted_at IS NULL
	`, invite.Code, patientID).Scan(&caredCount)
	if err != nil {
		t.Fatalf("count cared memberships: %v", err)
	}
	if caredCount != 1 {
		t.Fatalf("cared membership count = %d, want 1", caredCount)
	}

	var usedBy string
	var usageCount int
	err = pool.QueryRow(ctx, `
		SELECT used_by_user_id::text, usage_count
		FROM family_invites
		WHERE code = $1
	`, invite.Code).Scan(&usedBy, &usageCount)
	if err != nil {
		t.Fatalf("load invite usage: %v", err)
	}
	if usedBy != patientID {
		t.Fatalf("used_by_user_id = %q, want %q", usedBy, patientID)
	}
	if usageCount != 1 {
		t.Fatalf("usage_count = %d, want 1", usageCount)
	}
}

func TestIntegration_NewlyAuthedCaredUserCanJoinFamilyInvite(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerID := insertIntegrationUser(t, ctx, pool, "family")
	patientID := insertIntegrationUser(t, ctx, pool, "user")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerID, patientID)
	})

	invite, err := svc.CreateInvite(ctx, ownerID, "Gia dinh")
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	if err := svc.JoinFamily(ctx, patientID, invite.Code); err != nil {
		t.Fatalf("JoinFamily() error = %v", err)
	}
}

func TestIntegration_JoinFamilyRequiresLeavingCurrentGroup(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerA := insertIntegrationUser(t, ctx, pool, "family")
	ownerB := insertIntegrationUser(t, ctx, pool, "family")
	joiner := insertIntegrationUser(t, ctx, pool, "family")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerA, ownerB, joiner)
	})

	inviteA, err := svc.CreateInvite(ctx, ownerA, "Nhom A")
	if err != nil {
		t.Fatalf("CreateInvite(ownerA) error = %v", err)
	}
	inviteB, err := svc.CreateInvite(ctx, ownerB, "Nhom B")
	if err != nil {
		t.Fatalf("CreateInvite(ownerB) error = %v", err)
	}
	if err := svc.JoinFamily(ctx, joiner, inviteA.Code); err != nil {
		t.Fatalf("JoinFamily(inviteA) error = %v", err)
	}

	err = svc.JoinFamily(ctx, joiner, inviteB.Code)
	if err == nil {
		t.Fatal("expected join to second group to fail")
	}
	if !errors.Is(err, ErrGroupJoinRequiresLeaveCurrent) {
		t.Fatalf("JoinFamily(second group) error = %v, want %v", err, ErrGroupJoinRequiresLeaveCurrent)
	}
}

func TestIntegration_DeleteLastFamilyLinkAllowsJoiningAnotherGroup(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerA := insertIntegrationUser(t, ctx, pool, "family")
	ownerB := insertIntegrationUser(t, ctx, pool, "family")
	patient := insertIntegrationUser(t, ctx, pool, "user")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerA, ownerB, patient)
	})

	inviteA, err := svc.CreateInvite(ctx, ownerA, "Nhom A")
	if err != nil {
		t.Fatalf("CreateInvite(ownerA) error = %v", err)
	}
	if err := svc.JoinFamily(ctx, patient, inviteA.Code); err != nil {
		t.Fatalf("JoinFamily(patient -> A) error = %v", err)
	}

	var linkID string
	if err := pool.QueryRow(ctx, `
		SELECT id::text
		FROM family_links
		WHERE user_id = $1::uuid
		  AND family_id = $2::uuid
		  AND deleted_at IS NULL
	`, patient, ownerA).Scan(&linkID); err != nil {
		t.Fatalf("lookup family_link patient->ownerA: %v", err)
	}

	if err := svc.DeleteFamilyLink(ctx, patient, linkID); err != nil {
		t.Fatalf("DeleteFamilyLink() error = %v", err)
	}

	var liveCaredUsers int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM care_group_cared_users
		WHERE user_id = $1::uuid
		  AND deleted_at IS NULL
	`, patient).Scan(&liveCaredUsers); err != nil {
		t.Fatalf("count care_group_cared_users: %v", err)
	}
	if liveCaredUsers != 0 {
		t.Fatalf("live care_group_cared_users for patient = %d, want 0", liveCaredUsers)
	}

	inviteB, err := svc.CreateInvite(ctx, ownerB, "Nhom B")
	if err != nil {
		t.Fatalf("CreateInvite(ownerB) error = %v", err)
	}
	if err := svc.JoinFamily(ctx, patient, inviteB.Code); err != nil {
		t.Fatalf("JoinFamily(patient -> B) error = %v", err)
	}
}

func TestIntegration_TransferOwnershipThenOwnerCanLeave(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerID := insertIntegrationUser(t, ctx, pool, "family")
	memberID := insertIntegrationUser(t, ctx, pool, "family")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerID, memberID)
	})

	invite, err := svc.CreateInvite(ctx, ownerID, "Gia dinh")
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	if err := svc.JoinFamily(ctx, memberID, invite.Code); err != nil {
		t.Fatalf("JoinFamily(member) error = %v", err)
	}

	if err := svc.LeaveGroup(ctx, ownerID); !errors.Is(err, ErrGroupOwnerTransferRequired) {
		t.Fatalf("LeaveGroup(owner before transfer) error = %v, want %v", err, ErrGroupOwnerTransferRequired)
	}

	if err := svc.TransferGroupOwnership(ctx, ownerID, memberID); err != nil {
		t.Fatalf("TransferGroupOwnership() error = %v", err)
	}

	if err := svc.LeaveGroup(ctx, ownerID); err != nil {
		t.Fatalf("LeaveGroup(owner after transfer) error = %v", err)
	}

	var storedOwnerID string
	err = pool.QueryRow(ctx, `
		SELECT owner_user_id::text
		FROM care_groups
		WHERE owner_user_id = $1::uuid
		   OR owner_user_id = $2::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`, ownerID, memberID).Scan(&storedOwnerID)
	if err != nil {
		t.Fatalf("load care_groups owner: %v", err)
	}
	if storedOwnerID != memberID {
		t.Fatalf("owner_user_id = %q, want %q", storedOwnerID, memberID)
	}
}

func TestIntegration_GetCurrentGroupIncludesMembersAndCaredUsers(t *testing.T) {
	ctx := context.Background()
	pool := testIntegrationPool(t)
	svc := NewService(pool)

	ownerID := insertIntegrationUser(t, ctx, pool, "family")
	memberID := insertIntegrationUser(t, ctx, pool, "family")
	patientID := insertIntegrationUser(t, ctx, pool, "user")
	t.Cleanup(func() {
		cleanupIntegrationUsers(t, ctx, pool, ownerID, memberID, patientID)
	})

	memberInvite, err := svc.CreateInvite(ctx, ownerID, "Nguoi nha")
	if err != nil {
		t.Fatalf("CreateInvite(member) error = %v", err)
	}
	if err := svc.JoinFamily(ctx, memberID, memberInvite.Code); err != nil {
		t.Fatalf("JoinFamily(member) error = %v", err)
	}

	patientInvite, err := svc.CreateInvite(ctx, ownerID, "Nguoi benh")
	if err != nil {
		t.Fatalf("CreateInvite(patient) error = %v", err)
	}
	if err := svc.JoinFamily(ctx, patientID, patientInvite.Code); err != nil {
		t.Fatalf("JoinFamily(patient) error = %v", err)
	}

	group, err := svc.GetCurrentGroup(ctx, ownerID)
	if err != nil {
		t.Fatalf("GetCurrentGroup() error = %v", err)
	}
	if group == nil {
		t.Fatal("expected active group")
	}
	if !group.CanManageGroup {
		t.Fatal("owner should be able to manage group")
	}
	if len(group.FamilyMembers) != 2 {
		t.Fatalf("family member count = %d, want 2", len(group.FamilyMembers))
	}
	if len(group.CaredUsers) != 1 {
		t.Fatalf("cared user count = %d, want 1", len(group.CaredUsers))
	}
	if group.FamilyMembers[0].GroupRole == "" {
		t.Fatal("expected family member group role to be populated")
	}
	if group.CaredUsers[0].GroupRole != "cared_user" {
		t.Fatalf("cared user group role = %q, want cared_user", group.CaredUsers[0].GroupRole)
	}
}
