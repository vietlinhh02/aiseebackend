package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
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
