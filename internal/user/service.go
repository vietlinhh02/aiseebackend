package user

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"golang.org/x/text/unicode/norm"
)

type Service struct {
	queries *dbgen.Queries
	db      *pgxpool.Pool
	invites map[string]InviteData // In-memory for MVP, Redis later
}

type InviteData struct {
	UserID       string
	Relationship string
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		queries: dbgen.New(db),
		db:      db,
		invites: make(map[string]InviteData),
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

func (s *Service) CreateInvite(ctx context.Context, userID, relationship string) string {
	code := GenerateInviteCode()
	s.invites[code] = InviteData{UserID: userID, Relationship: relationship}
	return code
}

func (s *Service) JoinFamily(ctx context.Context, familyMemberID, code string) error {
	invite, ok := s.invites[code]
	if !ok {
		return fmt.Errorf("invalid invite code")
	}
	delete(s.invites, code)

	uid, err := parsePgUUID(invite.UserID)
	if err != nil {
		return err
	}
	fid, err := parsePgUUID(familyMemberID)
	if err != nil {
		return err
	}

	_, err = s.queries.CreateFamilyLink(ctx, dbgen.CreateFamilyLinkParams{
		UserID:           uid,
		FamilyID:         fid,
		Relationship:     invite.Relationship,
		IsPrimaryContact: false,
	})
	return err
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
	ctx context.Context, linkID string,
) error {
	lid, err := pgutil.ParsePgUUID(linkID)
	if err != nil {
		return fmt.Errorf("invalid link id: %w", err)
	}
	return s.queries.DeleteFamilyLink(ctx, lid)
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
