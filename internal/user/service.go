package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"golang.org/x/text/unicode/norm"
)

type Service struct {
	queries *dbgen.Queries
	db      *pgxpool.Pool
}

const defaultFamilyInviteTTL = 7 * 24 * time.Hour

var (
	ErrInviteCreateForbidden         = errors.New("invite creation requires user role")
	ErrInviteRevokeForbidden         = errors.New("cannot revoke this invite")
	ErrInviteInvalid                 = errors.New("invalid invite code")
	ErrInviteExpired                 = errors.New("invite code expired")
	ErrInviteAlreadyUsed             = errors.New("invite code already used")
	ErrFamilyLimitReached            = errors.New("family members limit reached")
	ErrCaredUserLimitReached         = errors.New("cared users limit reached")
	ErrDeleteFamilyForbidden         = errors.New("cannot delete this family link")
	ErrNoActiveGroup                 = errors.New("no active group")
	ErrGroupOwnerOnly                = errors.New("only the group owner can perform this action")
	ErrGroupJoinRequiresLeaveCurrent = errors.New("leave the current group before joining a new one")
	ErrGroupMemberAlreadyJoined      = errors.New("user already belongs to this group")
	ErrGroupOwnerTransferRequired    = errors.New("transfer group ownership before leaving the group")
)

type Invite struct {
	Code      string    `json:"invite_code"`
	ExpiresAt time.Time `json:"expires_at"`
	Reusable  bool      `json:"reusable"`
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		queries: dbgen.New(db),
		db:      db,
	}
}

func parsePgUUID(s string) (pgtype.UUID, error) {
	uid, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid user id: %w", err)
	}
	return pgtype.UUID{Bytes: uid, Valid: true}, nil
}

func (s *Service) GetMe(ctx context.Context, userID string) (dbgen.User, error) {
	uid, err := parsePgUUID(userID)
	if err != nil {
		return dbgen.User{}, err
	}
	return s.queries.GetUserByID(ctx, uid)
}

func (s *Service) GetFamily(ctx context.Context, userID string) ([]dbgen.User, error) {
	uid, err := parsePgUUID(userID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetFamilyMembers(ctx, uid)
}

type CareGroupPerson struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	AccountRole string `json:"account_role"`
	GroupRole   string `json:"group_role"`
}

type CurrentGroup struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	OwnerUserID      string            `json:"owner_user_id"`
	CurrentUserRole  string            `json:"current_user_role"`
	CanManageGroup   bool              `json:"can_manage_group"`
	CanManageBilling bool              `json:"can_manage_billing"`
	FamilyMembers    []CareGroupPerson `json:"family_members"`
	CaredUsers       []CareGroupPerson `json:"cared_users"`
}

func (s *Service) GetCurrentGroup(
	ctx context.Context,
	userID string,
) (*CurrentGroup, error) {
	state, err := s.activeGroupForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if state.CareGroupID == "" {
		return nil, ErrNoActiveGroup
	}

	group := &CurrentGroup{
		ID:               state.CareGroupID,
		CurrentUserRole:  state.MemberRole,
		CanManageGroup:   !state.IsCaredUser && state.MemberRole == "owner",
		CanManageBilling: !state.IsCaredUser && state.MemberRole == "owner",
	}
	if state.IsCaredUser {
		group.CurrentUserRole = "cared_user"
	}

	if err := s.db.QueryRow(ctx, `
		SELECT id::text, name, owner_user_id::text
		FROM care_groups
		WHERE id = $1::uuid
		  AND status = 'active'
	`, state.CareGroupID).Scan(&group.ID, &group.Name, &group.OwnerUserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoActiveGroup
		}
		return nil, fmt.Errorf("load current group: %w", err)
	}

	familyRows, err := s.db.Query(ctx, `
		SELECT
			u.id::text,
			u.email,
			COALESCE(u.display_name, ''),
			COALESCE(u.avatar_url, ''),
			u.role,
			cgm.role
		FROM care_group_members cgm
		JOIN users u ON u.id = cgm.user_id
		WHERE cgm.care_group_id = $1::uuid
		  AND cgm.deleted_at IS NULL
		ORDER BY
			CASE cgm.role
				WHEN 'owner' THEN 0
				WHEN 'billing_admin' THEN 1
				ELSE 2
			END,
			COALESCE(u.display_name, ''),
			u.email
	`, state.CareGroupID)
	if err != nil {
		return nil, fmt.Errorf("list family group members: %w", err)
	}
	defer familyRows.Close()

	for familyRows.Next() {
		var member CareGroupPerson
		if err := familyRows.Scan(
			&member.ID,
			&member.Email,
			&member.DisplayName,
			&member.AvatarURL,
			&member.AccountRole,
			&member.GroupRole,
		); err != nil {
			return nil, fmt.Errorf("scan family group member: %w", err)
		}
		group.FamilyMembers = append(group.FamilyMembers, member)
	}
	if err := familyRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate family group members: %w", err)
	}

	caredRows, err := s.db.Query(ctx, `
		SELECT
			u.id::text,
			u.email,
			COALESCE(u.display_name, ''),
			COALESCE(u.avatar_url, ''),
			u.role
		FROM care_group_cared_users cgu
		JOIN users u ON u.id = cgu.user_id
		WHERE cgu.care_group_id = $1::uuid
		  AND cgu.deleted_at IS NULL
		ORDER BY COALESCE(u.display_name, ''), u.email
	`, state.CareGroupID)
	if err != nil {
		return nil, fmt.Errorf("list cared users: %w", err)
	}
	defer caredRows.Close()

	for caredRows.Next() {
		var caredUser CareGroupPerson
		caredUser.GroupRole = "cared_user"
		if err := caredRows.Scan(
			&caredUser.ID,
			&caredUser.Email,
			&caredUser.DisplayName,
			&caredUser.AvatarURL,
			&caredUser.AccountRole,
		); err != nil {
			return nil, fmt.Errorf("scan cared user: %w", err)
		}
		group.CaredUsers = append(group.CaredUsers, caredUser)
	}
	if err := caredRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cared users: %w", err)
	}

	return group, nil
}

func (s *Service) CreateInvite(
	ctx context.Context,
	userID, relationship string,
) (Invite, error) {
	relationship = strings.TrimSpace(relationship)
	if relationship == "" {
		relationship = "Gia dinh"
	}
	role, err := s.userRole(ctx, userID)
	if err != nil {
		return Invite{}, err
	}
	if role != "family" {
		return Invite{}, ErrInviteCreateForbidden
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Invite{}, fmt.Errorf("begin create invite tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	careGroupID, err := s.ensureOwnedCareGroupForFamilyTx(ctx, tx, userID)
	if err != nil {
		return Invite{}, err
	}
	expiresAt := time.Now().UTC().Add(defaultFamilyInviteTTL)
	for i := 0; i < 5; i++ {
		code := GenerateInviteCode()
		_, err := tx.Exec(ctx, `
			INSERT INTO family_invites (
				code, cared_user_id, care_group_id, owner_user_id, relationship,
				created_by_user_id, expires_at, max_uses
			) VALUES (
				$1, NULL, $2::uuid, $3::uuid, $4, $3::uuid, $5, 1
			)
		`, code, careGroupID, userID, relationship, expiresAt)
		if err == nil {
			if err := tx.Commit(ctx); err != nil {
				return Invite{}, fmt.Errorf("commit create invite: %w", err)
			}
			return Invite{
				Code:      code,
				ExpiresAt: expiresAt,
				Reusable:  false,
			}, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return Invite{}, fmt.Errorf("create invite: %w", err)
	}
	return Invite{}, fmt.Errorf("create invite: could not allocate unique code")
}

func (s *Service) JoinFamily(ctx context.Context, joinerID, code string) error {
	code = normalizeInviteCode(code)
	if code == "" {
		return ErrInviteInvalid
	}
	joinerRole, err := s.userRole(ctx, joinerID)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin join family tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	invite, err := s.familyInviteForUpdate(ctx, tx, code)
	if err != nil {
		return err
	}
	groupState, err := s.activeGroupForUserTx(ctx, tx, joinerID)
	if err != nil {
		return err
	}
	if groupState.CareGroupID != "" {
		if groupState.CareGroupID == invite.CareGroupID {
			return ErrGroupMemberAlreadyJoined
		}
		return ErrGroupJoinRequiresLeaveCurrent
	}

	switch joinerRole {
	case "user":
		if err := s.addCaredUserToGroupTx(ctx, tx, invite.CareGroupID, joinerID); err != nil {
			return err
		}
	case "family":
		if err := s.addFamilyMemberToGroupTx(ctx, tx, invite.CareGroupID, joinerID, "family"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported joiner role: %s", joinerRole)
	}

	if err := s.markInviteUsedTx(ctx, tx, invite.ID, joinerID); err != nil {
		return err
	}
	if err := s.syncCompatibilityLinksForJoinTx(
		ctx, tx, invite.CareGroupID, invite.OwnerUserID, joinerID, joinerRole, invite.Relationship,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) RevokeInvite(
	ctx context.Context,
	requesterID string,
	code string,
) error {
	code = normalizeInviteCode(code)
	if code == "" {
		return ErrInviteInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revoke invite tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerUserID, createdByUserID string
	var revokedAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		SELECT owner_user_id::text, created_by_user_id::text, revoked_at
		FROM family_invites
		WHERE code = $1
		FOR UPDATE
	`, code).Scan(&ownerUserID, &createdByUserID, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInviteInvalid
		}
		return fmt.Errorf("lookup invite for revoke: %w", err)
	}
	if requesterID != ownerUserID && requesterID != createdByUserID {
		return ErrInviteRevokeForbidden
	}
	if revokedAt.Valid {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE family_invites
		SET revoked_at = now(),
			updated_at = now()
		WHERE code = $1
	`, code); err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Service) ensureCareGroupLink(
	ctx context.Context,
	caredUserID string,
	familyMemberID string,
	primary bool,
) error {
	role := "family"
	if primary {
		role = "billing_admin"
	}
	_, err := s.db.Exec(ctx, `
		WITH existing_group AS (
			SELECT id
			FROM care_groups
			WHERE owner_user_id = $1::uuid
			  AND status = 'active'
			ORDER BY created_at
			LIMIT 1
		),
		created_group AS (
			INSERT INTO care_groups (name, owner_user_id)
			SELECT 'AiSee Care Group', $1::uuid
			WHERE NOT EXISTS (SELECT 1 FROM existing_group)
			RETURNING id
		),
		selected_group AS (
			SELECT id FROM existing_group
			UNION ALL
			SELECT id FROM created_group
			LIMIT 1
		),
		insert_cared AS (
			INSERT INTO care_group_cared_users (care_group_id, user_id)
			SELECT id, $1::uuid FROM selected_group
			ON CONFLICT (care_group_id, user_id)
			DO UPDATE SET deleted_at = NULL
			RETURNING 1
		)
		INSERT INTO care_group_members (care_group_id, user_id, role)
		SELECT id, $2::uuid, $3 FROM selected_group
		ON CONFLICT (care_group_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, deleted_at = NULL
	`, caredUserID, familyMemberID, role)
	if err != nil {
		return fmt.Errorf("ensure care group link: %w", err)
	}
	return nil
}

type familyInviteRecord struct {
	ID           string
	CareGroupID  string
	OwnerUserID  string
	Relationship string
	ExpiresAt    time.Time
	MaxUses      *int32
	UsageCount   int32
	UsedByUserID string
}

type familyLinkState struct {
	Active           bool
	IsPrimaryContact bool
}

type careGroupCapacity struct {
	ActiveMemberCount    int
	ActiveCaredUserCount int
	FamilyMemberLimit    int
	CaredUserLimit       int
	HasPrimaryContact    bool
	JoinerActiveMember   bool
}

func (s *Service) familyInviteForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	code string,
) (familyInviteRecord, error) {
	var invite familyInviteRecord
	var maxUses pgtype.Int4
	err := tx.QueryRow(ctx, `
		SELECT
			id::text,
			care_group_id::text,
			owner_user_id::text,
			relationship,
			expires_at,
			max_uses,
			usage_count,
			COALESCE(used_by_user_id::text, '')
		FROM family_invites
		WHERE code = $1
		  AND revoked_at IS NULL
		FOR UPDATE
	`, code).Scan(
		&invite.ID,
		&invite.CareGroupID,
		&invite.OwnerUserID,
		&invite.Relationship,
		&invite.ExpiresAt,
		&maxUses,
		&invite.UsageCount,
		&invite.UsedByUserID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return familyInviteRecord{}, ErrInviteInvalid
		}
		return familyInviteRecord{}, fmt.Errorf("lookup invite: %w", err)
	}
	if invite.ExpiresAt.Before(time.Now().UTC()) {
		return familyInviteRecord{}, ErrInviteExpired
	}
	if invite.UsedByUserID != "" {
		return familyInviteRecord{}, ErrInviteAlreadyUsed
	}
	if maxUses.Valid {
		value := maxUses.Int32
		invite.MaxUses = &value
		if invite.UsageCount >= value {
			return familyInviteRecord{}, ErrInviteAlreadyUsed
		}
	}
	return invite, nil
}

type activeGroupMembership struct {
	CareGroupID string
	MemberRole  string
	IsCaredUser bool
}

func (s *Service) activeGroupForUserTx(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
) (activeGroupMembership, error) {
	var state activeGroupMembership
	err := tx.QueryRow(ctx, `
		WITH memberships AS (
			SELECT cgm.care_group_id::text, cgm.role, false AS is_cared_user, 0 AS priority
			FROM care_group_members cgm
			JOIN care_groups cg ON cg.id = cgm.care_group_id
			WHERE cgm.user_id = $1::uuid
			  AND cgm.deleted_at IS NULL
			  AND cg.status = 'active'

			UNION ALL

			SELECT cgu.care_group_id::text, 'cared_user' AS role, true AS is_cared_user, 1 AS priority
			FROM care_group_cared_users cgu
			JOIN care_groups cg ON cg.id = cgu.care_group_id
			WHERE cgu.user_id = $1::uuid
			  AND cgu.deleted_at IS NULL
			  AND cg.status = 'active'
		)
		SELECT care_group_id, role, is_cared_user
		FROM memberships
		ORDER BY priority, care_group_id
		LIMIT 1
	`, userID).Scan(&state.CareGroupID, &state.MemberRole, &state.IsCaredUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return activeGroupMembership{}, nil
		}
		return activeGroupMembership{}, fmt.Errorf("active group for user: %w", err)
	}
	return state, nil
}

func (s *Service) activeGroupForUser(
	ctx context.Context,
	userID string,
) (activeGroupMembership, error) {
	var state activeGroupMembership
	err := s.db.QueryRow(ctx, `
		WITH memberships AS (
			SELECT cgm.care_group_id::text, cgm.role, false AS is_cared_user, 0 AS priority
			FROM care_group_members cgm
			JOIN care_groups cg ON cg.id = cgm.care_group_id
			WHERE cgm.user_id = $1::uuid
			  AND cgm.deleted_at IS NULL
			  AND cg.status = 'active'

			UNION ALL

			SELECT cgu.care_group_id::text, 'cared_user' AS role, true AS is_cared_user, 1 AS priority
			FROM care_group_cared_users cgu
			JOIN care_groups cg ON cg.id = cgu.care_group_id
			WHERE cgu.user_id = $1::uuid
			  AND cgu.deleted_at IS NULL
			  AND cg.status = 'active'
		)
		SELECT care_group_id, role, is_cared_user
		FROM memberships
		ORDER BY priority, care_group_id
		LIMIT 1
	`, userID).Scan(&state.CareGroupID, &state.MemberRole, &state.IsCaredUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return activeGroupMembership{}, nil
		}
		return activeGroupMembership{}, fmt.Errorf("active group for user: %w", err)
	}
	return state, nil
}

func (s *Service) ensureOwnedCareGroupForFamilyTx(
	ctx context.Context,
	tx pgx.Tx,
	familyUserID string,
) (string, error) {
	state, err := s.activeGroupForUserTx(ctx, tx, familyUserID)
	if err != nil {
		return "", err
	}
	if state.CareGroupID != "" {
		if state.IsCaredUser || state.MemberRole != "owner" {
			return "", ErrGroupOwnerOnly
		}
		return state.CareGroupID, nil
	}

	var careGroupID string
	err = tx.QueryRow(ctx, `
		WITH created_group AS (
			INSERT INTO care_groups (name, owner_user_id)
			VALUES ('AiSee Care Group', $1::uuid)
			RETURNING id
		),
		owner_member AS (
			INSERT INTO care_group_members (care_group_id, user_id, role)
			SELECT id, $1::uuid, 'owner'
			FROM created_group
			RETURNING 1
		)
		SELECT id::text FROM created_group
	`, familyUserID).Scan(&careGroupID)
	if err != nil {
		return "", fmt.Errorf("create owned care group: %w", err)
	}
	return careGroupID, nil
}

func (s *Service) addCaredUserToGroupTx(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
	userID string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO care_group_cared_users (care_group_id, user_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (care_group_id, user_id)
		DO UPDATE SET deleted_at = NULL
	`, careGroupID, userID)
	if err != nil {
		return fmt.Errorf("add cared user to group: %w", err)
	}
	return nil
}

func (s *Service) addFamilyMemberToGroupTx(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
	userID string,
	role string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO care_group_members (care_group_id, user_id, role)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (care_group_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, deleted_at = NULL
	`, careGroupID, userID, role)
	if err != nil {
		return fmt.Errorf("add family member to group: %w", err)
	}
	return nil
}

func (s *Service) markInviteUsedTx(
	ctx context.Context,
	tx pgx.Tx,
	inviteID string,
	usedByUserID string,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE family_invites
		SET usage_count = usage_count + 1,
			used_by_user_id = $2::uuid,
			used_at = now(),
			last_used_at = now(),
			updated_at = now()
		WHERE id = $1::uuid
	`, inviteID, usedByUserID)
	if err != nil {
		return fmt.Errorf("mark invite used: %w", err)
	}
	return nil
}

func (s *Service) syncCompatibilityLinksForJoinTx(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
	ownerUserID string,
	joinerID string,
	joinerRole string,
	relationship string,
) error {
	if relationship == "" {
		relationship = "Gia dinh"
	}
	switch joinerRole {
	case "user":
		rows, err := tx.Query(ctx, `
			SELECT user_id::text, role
			FROM care_group_members
			WHERE care_group_id = $1::uuid
			  AND deleted_at IS NULL
		`, careGroupID)
		if err != nil {
			return fmt.Errorf("list family members for compatibility links: %w", err)
		}
		type familyMemberRow struct {
			userID string
			role   string
		}
		var familyMembers []familyMemberRow
		for rows.Next() {
			var familyID, role string
			if err := rows.Scan(&familyID, &role); err != nil {
				rows.Close()
				return fmt.Errorf("scan family member for compatibility links: %w", err)
			}
			familyMembers = append(familyMembers, familyMemberRow{
				userID: familyID,
				role:   role,
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate family members for compatibility links: %w", err)
		}
		rows.Close()
		for _, familyMember := range familyMembers {
			if err := s.upsertFamilyLinkTx(
				ctx, tx, joinerID, familyMember.userID, relationship,
				familyMember.userID == ownerUserID || familyMember.role == "owner",
			); err != nil {
				return err
			}
		}
	case "family":
		rows, err := tx.Query(ctx, `
			SELECT user_id::text
			FROM care_group_cared_users
			WHERE care_group_id = $1::uuid
			  AND deleted_at IS NULL
		`, careGroupID)
		if err != nil {
			return fmt.Errorf("list cared users for compatibility links: %w", err)
		}
		var caredUserIDs []string
		for rows.Next() {
			var caredUserID string
			if err := rows.Scan(&caredUserID); err != nil {
				rows.Close()
				return fmt.Errorf("scan cared user for compatibility links: %w", err)
			}
			caredUserIDs = append(caredUserIDs, caredUserID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate cared users for compatibility links: %w", err)
		}
		rows.Close()
		for _, caredUserID := range caredUserIDs {
			if err := s.upsertFamilyLinkTx(
				ctx, tx, caredUserID, joinerID, relationship, joinerID == ownerUserID,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ensureCareGroupForCaredUser(
	ctx context.Context,
	tx pgx.Tx,
	caredUserID string,
) (string, error) {
	var careGroupID string
	err := tx.QueryRow(ctx, `
		WITH owned AS (
			SELECT id
			FROM care_groups
			WHERE owner_user_id = $1::uuid
			  AND status = 'active'
			ORDER BY created_at
			LIMIT 1
		),
		created AS (
			INSERT INTO care_groups (name, owner_user_id)
			SELECT 'AiSee Care Group', $1::uuid
			WHERE NOT EXISTS (SELECT 1 FROM owned)
			RETURNING id
		),
		selected AS (
			SELECT id FROM owned
			UNION ALL
			SELECT id FROM created
			LIMIT 1
		),
		owner_member AS (
			INSERT INTO care_group_members (care_group_id, user_id, role)
			SELECT id, $1::uuid, 'owner' FROM selected
			ON CONFLICT (care_group_id, user_id)
			DO UPDATE SET role = 'owner', deleted_at = NULL
			RETURNING 1
		),
		cared_upsert AS (
			INSERT INTO care_group_cared_users (care_group_id, user_id)
			SELECT id, $1::uuid FROM selected
			ON CONFLICT (care_group_id, user_id)
			DO UPDATE SET deleted_at = NULL
			RETURNING 1
		)
		SELECT id::text FROM selected
	`, caredUserID).Scan(&careGroupID)
	if err != nil {
		return "", fmt.Errorf("ensure care group: %w", err)
	}
	return careGroupID, nil
}

func (s *Service) lookupFamilyLinkState(
	ctx context.Context,
	tx pgx.Tx,
	caredUserID string,
	familyMemberID string,
) (familyLinkState, error) {
	var state familyLinkState
	var deletedAt pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		SELECT is_primary_contact, deleted_at
		FROM family_links
		WHERE user_id = $1::uuid
		  AND family_id = $2::uuid
	`, caredUserID, familyMemberID).Scan(&state.IsPrimaryContact, &deletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return familyLinkState{}, nil
		}
		return familyLinkState{}, fmt.Errorf("lookup family link: %w", err)
	}
	state.Active = !deletedAt.Valid
	return state, nil
}

func (s *Service) careGroupCapacity(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
	caredUserID string,
	familyMemberID string,
) (careGroupCapacity, error) {
	var capacity careGroupCapacity
	err := tx.QueryRow(ctx, `
		WITH plan_choice AS (
			SELECT COALESCE(
				(
					SELECT s.plan_id
					FROM subscriptions s
					JOIN plans p ON p.id = s.plan_id
					WHERE s.care_group_id = $1::uuid
					  AND s.status IN ('trialing', 'active', 'past_due')
					  AND p.status = 'active'
					ORDER BY
						CASE s.status
							WHEN 'active' THEN 1
							WHEN 'trialing' THEN 2
							ELSE 3
						END,
						p.sort_order DESC,
						p.price_vnd DESC,
						s.created_at DESC
					LIMIT 1
				),
				(
					SELECT p.id
					FROM plans p
					WHERE p.status = 'active'
					ORDER BY p.sort_order, p.price_vnd, p.code
					LIMIT 1
				)
			) AS plan_id
		)
		SELECT
			COALESCE((
				SELECT NULLIF(trim(both '"' from pe.limit_value::text), '')::int
				FROM plan_choice pc
				JOIN plan_entitlements pe ON pe.plan_id = pc.plan_id
				WHERE pe.feature_key = 'family_members_limit'
				LIMIT 1
			), 0),
			COALESCE((
				SELECT NULLIF(trim(both '"' from pe.limit_value::text), '')::int
				FROM plan_choice pc
				JOIN plan_entitlements pe ON pe.plan_id = pc.plan_id
				WHERE pe.feature_key = 'cared_users_limit'
				LIMIT 1
			), 0),
			(
				SELECT COUNT(*)
				FROM care_group_members
				WHERE care_group_id = $1::uuid
				  AND role <> 'owner'
				  AND deleted_at IS NULL
			)::int,
			(
				SELECT COUNT(*)
				FROM care_group_cared_users
				WHERE care_group_id = $1::uuid
				  AND deleted_at IS NULL
			)::int,
			EXISTS(
				SELECT 1
				FROM family_links
				WHERE user_id = $2::uuid
				  AND deleted_at IS NULL
				  AND is_primary_contact = true
			),
			EXISTS(
				SELECT 1
				FROM care_group_members
				WHERE care_group_id = $1::uuid
				  AND user_id = $3::uuid
				  AND deleted_at IS NULL
			)
	`, careGroupID, caredUserID, familyMemberID).Scan(
		&capacity.FamilyMemberLimit,
		&capacity.CaredUserLimit,
		&capacity.ActiveMemberCount,
		&capacity.ActiveCaredUserCount,
		&capacity.HasPrimaryContact,
		&capacity.JoinerActiveMember,
	)
	if err != nil {
		return careGroupCapacity{}, fmt.Errorf("care group capacity: %w", err)
	}
	return capacity, nil
}

func (s *Service) lockCareGroupForCapacityTx(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
) error {
	var lockedID string
	if err := tx.QueryRow(ctx, `
		SELECT id::text
		FROM care_groups
		WHERE id = $1::uuid
		FOR UPDATE
	`, careGroupID).Scan(&lockedID); err != nil {
		return fmt.Errorf("lock care group for capacity: %w", err)
	}
	return nil
}

func nextFamilyInviteRole(hasPrimaryContact bool) (bool, string) {
	if !hasPrimaryContact {
		return true, "billing_admin"
	}
	return false, "family"
}

func normalizeInviteCode(code string) string {
	return strings.TrimSpace(strings.ToLower(code))
}

func (s *Service) upsertFamilyLinkTx(
	ctx context.Context,
	tx pgx.Tx,
	caredUserID string,
	familyMemberID string,
	relationship string,
	isPrimaryContact bool,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO family_links (user_id, family_id, relationship, is_primary_contact)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		ON CONFLICT (user_id, family_id)
		DO UPDATE SET
			relationship = EXCLUDED.relationship,
			is_primary_contact = EXCLUDED.is_primary_contact,
			deleted_at = NULL
	`, caredUserID, familyMemberID, relationship, isPrimaryContact)
	if err != nil {
		return fmt.Errorf("upsert family link: %w", err)
	}
	return nil
}

func (s *Service) ensureCareGroupMemberTx(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
	familyMemberID string,
	role string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO care_group_members (care_group_id, user_id, role)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (care_group_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, deleted_at = NULL
	`, careGroupID, familyMemberID, role)
	if err != nil {
		return fmt.Errorf("ensure care group member: %w", err)
	}
	return nil
}

func (s *Service) bumpInviteUsage(
	ctx context.Context,
	tx pgx.Tx,
	inviteID string,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE family_invites
		SET usage_count = usage_count + 1,
			last_used_at = now(),
			updated_at = now()
		WHERE id = $1::uuid
	`, inviteID)
	if err != nil {
		return fmt.Errorf("bump invite usage: %w", err)
	}
	return nil
}

// normalizeVietnamese lowercases and strips Vietnamese
// diacritics for fuzzy matching.
func normalizeVietnamese(s string) string {
	s = strings.ToLower(s)
	t := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range t {
		if !unicode.Is(unicode.Mn, r) {
			if r == 'đ' {
				b.WriteRune('d')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func matchContact(
	query, displayName, relationship string,
) bool {
	q := normalizeVietnamese(query)
	return strings.Contains(
		normalizeVietnamese(displayName), q,
	) || strings.Contains(
		normalizeVietnamese(relationship), q,
	)
}

type FamilyMember struct {
	User         dbgen.User `json:"user"`
	Relationship string     `json:"relationship"`
}

func (s *Service) ResolveContact(
	ctx context.Context, userID, contactName string,
) (FamilyMember, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return FamilyMember{}, err
	}

	rows, err := s.queries.GetFamilyMembersWithDetails(
		ctx, uid,
	)
	if err != nil {
		return FamilyMember{},
			fmt.Errorf("get family members: %w", err)
	}

	var matches []FamilyMember
	for _, row := range rows {
		member := FamilyMember{
			User: dbgen.User{
				ID:            row.ID,
				GoogleID:      row.GoogleID,
				Email:         row.Email,
				DisplayName:   row.DisplayName,
				Role:          row.Role,
				IsVolunteer:   row.IsVolunteer,
				AvatarUrl:     row.AvatarUrl,
				FcmToken:      row.FcmToken,
				ApnsToken:     row.ApnsToken,
				ApnsVoipToken: row.ApnsVoipToken,
				Settings:      row.Settings,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
			},
			Relationship: row.Relationship,
		}
		if matchContact(
			contactName, row.DisplayName, row.Relationship,
		) {
			matches = append(matches, member)
		}
	}

	switch len(matches) {
	case 0:
		return FamilyMember{},
			fmt.Errorf("contact not found: %s", contactName)
	case 1:
		return matches[0], nil
	default:
		return FamilyMember{}, fmt.Errorf(
			"multiple contacts match '%s', "+
				"please be more specific",
			contactName,
		)
	}
}

func (s *Service) DeleteFamilyLink(
	ctx context.Context, requesterID string, linkID string,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete family link tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var caredUserID, familyMemberID string
	var wasPrimary bool
	err = tx.QueryRow(ctx, `
		SELECT user_id::text, family_id::text, is_primary_contact
		FROM family_links
		WHERE id = $1::uuid
		  AND deleted_at IS NULL
	`, linkID).Scan(&caredUserID, &familyMemberID, &wasPrimary)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("invalid link id")
		}
		return fmt.Errorf("lookup family link: %w", err)
	}
	if requesterID != caredUserID && requesterID != familyMemberID {
		return ErrDeleteFamilyForbidden
	}
	if _, err := tx.Exec(ctx, `
		UPDATE family_links
		SET deleted_at = now()
		WHERE id = $1::uuid
		  AND deleted_at IS NULL
	`, linkID); err != nil {
		return fmt.Errorf("delete family link: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE care_group_members cgm
		SET deleted_at = now()
		WHERE cgm.user_id = $1::uuid
		  AND cgm.deleted_at IS NULL
		  AND EXISTS (
			SELECT 1
			FROM care_group_cared_users cgu
			JOIN care_groups cg ON cg.id = cgu.care_group_id
			WHERE cgu.care_group_id = cgm.care_group_id
			  AND cgu.user_id = $2::uuid
			  AND cgu.deleted_at IS NULL
			  AND cg.status = 'active'
		  )
	`, familyMemberID, caredUserID); err != nil {
		return fmt.Errorf("cleanup care group member: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE care_group_cared_users
		SET deleted_at = now()
		WHERE user_id = $1::uuid
		  AND deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM family_links fl
			WHERE fl.deleted_at IS NULL
			  AND (fl.user_id = $1::uuid OR fl.family_id = $1::uuid)
		  )
	`, caredUserID); err != nil {
		return fmt.Errorf("cleanup care group cared user: %w", err)
	}
	if wasPrimary {
		if err := s.rebalancePrimaryContactTx(ctx, tx, caredUserID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) LeaveGroup(
	ctx context.Context,
	userID string,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin leave group tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := s.activeGroupForUserTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if state.CareGroupID == "" {
		return tx.Commit(ctx)
	}
	if state.MemberRole == "owner" && !state.IsCaredUser {
		return ErrGroupOwnerTransferRequired
	}

	if state.IsCaredUser {
		if _, err := tx.Exec(ctx, `
			UPDATE care_group_cared_users
			SET deleted_at = now()
			WHERE care_group_id = $1::uuid
			  AND user_id = $2::uuid
			  AND deleted_at IS NULL
		`, state.CareGroupID, userID); err != nil {
			return fmt.Errorf("leave cared user group: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE family_links
			SET deleted_at = now()
			WHERE user_id = $1::uuid
			  AND deleted_at IS NULL
		`, userID); err != nil {
			return fmt.Errorf("delete cared user compatibility links: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE care_group_members
			SET deleted_at = now()
			WHERE care_group_id = $1::uuid
			  AND user_id = $2::uuid
			  AND deleted_at IS NULL
		`, state.CareGroupID, userID); err != nil {
			return fmt.Errorf("leave family member group: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE family_links
			SET deleted_at = now()
			WHERE family_id = $1::uuid
			  AND deleted_at IS NULL
		`, userID); err != nil {
			return fmt.Errorf("delete family compatibility links: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) TransferGroupOwnership(
	ctx context.Context,
	requesterID string,
	targetUserID string,
) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transfer ownership tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := s.activeGroupForUserTx(ctx, tx, requesterID)
	if err != nil {
		return err
	}
	if state.CareGroupID == "" || state.IsCaredUser || state.MemberRole != "owner" {
		return ErrGroupOwnerOnly
	}

	var targetExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM care_group_members
			WHERE care_group_id = $1::uuid
			  AND user_id = $2::uuid
			  AND deleted_at IS NULL
		)
	`, state.CareGroupID, targetUserID).Scan(&targetExists); err != nil {
		return fmt.Errorf("lookup transfer target: %w", err)
	}
	if !targetExists {
		return fmt.Errorf("target user must be an active family member in the group")
	}

	if _, err := tx.Exec(ctx, `
		UPDATE care_groups
		SET owner_user_id = $2::uuid,
			updated_at = now()
		WHERE id = $1::uuid
	`, state.CareGroupID, targetUserID); err != nil {
		return fmt.Errorf("update care group owner: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE care_group_members
		SET role = CASE
			WHEN user_id = $2::uuid THEN 'owner'
			WHEN user_id = $3::uuid THEN 'family'
			ELSE role
		END
		WHERE care_group_id = $1::uuid
		  AND user_id IN ($2::uuid, $3::uuid)
		  AND deleted_at IS NULL
	`, state.CareGroupID, targetUserID, requesterID); err != nil {
		return fmt.Errorf("update care group member roles: %w", err)
	}
	if err := s.syncOwnerPrimaryContactsTx(ctx, tx, state.CareGroupID, targetUserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) syncOwnerPrimaryContactsTx(
	ctx context.Context,
	tx pgx.Tx,
	careGroupID string,
	ownerUserID string,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE family_links fl
		SET is_primary_contact = (fl.family_id = $2::uuid)
		WHERE fl.deleted_at IS NULL
		  AND fl.user_id IN (
			SELECT user_id
			FROM care_group_cared_users
			WHERE care_group_id = $1::uuid
			  AND deleted_at IS NULL
		  )
		  AND fl.family_id IN (
			SELECT user_id
			FROM care_group_members
			WHERE care_group_id = $1::uuid
			  AND deleted_at IS NULL
		  )
	`, careGroupID, ownerUserID)
	if err != nil {
		return fmt.Errorf("sync owner primary contacts: %w", err)
	}
	return nil
}

func (s *Service) rebalancePrimaryContactTx(
	ctx context.Context,
	tx pgx.Tx,
	caredUserID string,
) error {
	var nextFamilyID string
	err := tx.QueryRow(ctx, `
		SELECT family_id::text
		FROM family_links
		WHERE user_id = $1::uuid
		  AND deleted_at IS NULL
		ORDER BY is_primary_contact DESC, created_at ASC, family_id
		LIMIT 1
	`, caredUserID).Scan(&nextFamilyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("select replacement primary contact: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE family_links
		SET is_primary_contact = (family_id = $2::uuid),
			deleted_at = deleted_at
		WHERE user_id = $1::uuid
		  AND deleted_at IS NULL
	`, caredUserID, nextFamilyID); err != nil {
		return fmt.Errorf("promote replacement primary contact: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE care_group_members cgm
		SET role = CASE
			WHEN cgm.user_id = $2::uuid THEN 'billing_admin'
			ELSE 'family'
		END
		WHERE cgm.deleted_at IS NULL
		  AND EXISTS (
			SELECT 1
			FROM care_group_cared_users cgu
			WHERE cgu.care_group_id = cgm.care_group_id
			  AND cgu.user_id = $1::uuid
			  AND cgu.deleted_at IS NULL
		  )
		  AND EXISTS (
			SELECT 1
			FROM family_links fl
			WHERE fl.user_id = $1::uuid
			  AND fl.family_id = cgm.user_id
			  AND fl.deleted_at IS NULL
		  )
	`, caredUserID, nextFamilyID); err != nil {
		return fmt.Errorf("sync replacement billing admin: %w", err)
	}
	return nil
}

func (s *Service) userRole(ctx context.Context, userID string) (string, error) {
	var role string
	if err := s.db.QueryRow(
		ctx, `SELECT role FROM users WHERE id = $1::uuid`, userID,
	).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("user not found")
		}
		return "", fmt.Errorf("lookup user role: %w", err)
	}
	return role, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Service) CheckFamilyLink(
	ctx context.Context, userID, otherID string,
) (bool, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return false, err
	}
	oid, err := pgutil.ParsePgUUID(otherID)
	if err != nil {
		return false, err
	}
	return s.queries.CheckFamilyLink(
		ctx, dbgen.CheckFamilyLinkParams{
			UserID:   uid,
			FamilyID: oid,
		},
	)
}

func (s *Service) UpdateProfile(
	ctx context.Context,
	userID string,
	req UpdateMeRequest,
) (dbgen.User, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.User{}, err
	}

	var avatarURL, fcmToken pgtype.Text
	var apnsToken, apnsVoipToken pgtype.Text
	if req.AvatarURL != "" {
		avatarURL = pgtype.Text{
			String: req.AvatarURL, Valid: true,
		}
	}
	if req.FcmToken != "" {
		fcmToken = pgtype.Text{
			String: req.FcmToken, Valid: true,
		}
	}
	if req.ApnsToken != "" {
		apnsToken = pgtype.Text{
			String: req.ApnsToken, Valid: true,
		}
	}
	if req.ApnsVoipToken != "" {
		apnsVoipToken = pgtype.Text{
			String: req.ApnsVoipToken, Valid: true,
		}
	}

	var settings []byte
	if req.Settings != nil {
		settings, err = json.Marshal(req.Settings)
		if err != nil {
			return dbgen.User{},
				fmt.Errorf("marshal settings: %w", err)
		}
	}

	return s.queries.UpdateUserProfile(
		ctx, dbgen.UpdateUserProfileParams{
			ID:            uid,
			Column2:       req.DisplayName,
			AvatarUrl:     avatarURL,
			FcmToken:      fcmToken,
			ApnsToken:     apnsToken,
			ApnsVoipToken: apnsVoipToken,
			Settings:      settings,
		},
	)
}
