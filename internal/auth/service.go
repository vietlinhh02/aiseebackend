package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

const (
	accessTokenTTL          = time.Hour
	refreshTokenTTL         = 10 * 365 * 24 * time.Hour
	accessTokenExpiresInSec = int64(accessTokenTTL / time.Second)
)

type Service struct {
	db        *pgxpool.Pool
	queries   *dbgen.Queries
	rdb       *redis.Client
	jwtSecret string
}

func NewService(
	db *pgxpool.Pool, rdb *redis.Client, jwtSecret string,
) *Service {
	return &Service{
		db:        db,
		queries:   dbgen.New(db),
		rdb:       rdb,
		jwtSecret: jwtSecret,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type tokenIssue struct {
	UserID           string
	Role             string
	SessionID        string
	RefreshJti       string
	RefreshExpiresAt time.Time
}

type refreshSessionRotation struct {
	UserID             string
	SessionID          string
	PreviousRefreshJti string
	RefreshJti         string
	ExpiresAt          time.Time
}

func (s *Service) AuthenticateGoogle(
	ctx context.Context,
	idToken, role string,
) (*TokenPair, error) {
	info, err := VerifyGoogleToken(ctx, idToken)
	if err != nil {
		return nil, err
	}

	user, err := s.queries.GetUserByGoogleID(ctx, info.Sub)
	if err != nil {
		user, err = s.queries.CreateUser(ctx, dbgen.CreateUserParams{
			GoogleID:    info.Sub,
			Email:       info.Email,
			DisplayName: info.Name,
			Role:        role,
		})
		if err != nil {
			return nil, err
		}
	} else if role != "" && user.Role != role {
		err = s.queries.UpdateUserRole(ctx, dbgen.UpdateUserRoleParams{
			ID:   user.ID,
			Role: role,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update user role: %w", err)
		}
		user.Role = role
	}

	effectiveRole := s.resolveEffectiveRole(ctx, user.ID.String(), user.Role)

	s.cacheUserStatus(ctx, user.ID.String(), effectiveRole)

	return s.generateTokenPair(ctx, user.ID.String(), effectiveRole)
}

func (s *Service) resolveEffectiveRole(ctx context.Context, userID, baseRole string) string {
	if s.db == nil {
		return baseRole
	}
	var isAdmin bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM admin_users WHERE user_id = $1)`,
		userID,
	).Scan(&isAdmin); err != nil || !isAdmin {
		return baseRole
	}
	return "admin"
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (any, error) {
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	typ, _ := claims["type"].(string)
	if typ != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("invalid claims")
	}
	role, _ := claims["role"].(string)
	sessionID, _ := claims["sid"].(string)
	previousRefreshJti, _ := claims["jti"].(string)
	var dbErr error
	role, dbErr = s.currentBaseRole(ctx, sub, role)
	if dbErr != nil {
		return nil, fmt.Errorf("failed to refresh role: %w", dbErr)
	}
	effectiveRole := s.resolveEffectiveRole(ctx, sub, role)

	return s.rotateTokenPair(ctx, sub, effectiveRole, sessionID, previousRefreshJti)
}

func (s *Service) currentBaseRole(ctx context.Context, userID, fallback string) (string, error) {
	if s.db == nil {
		return fallback, nil
	}
	var role string
	err := s.db.QueryRow(
		ctx,
		`SELECT role FROM users WHERE id = $1`,
		userID,
	).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *Service) cacheUserStatus(ctx context.Context, userID, role string) {
	if s.rdb == nil {
		return
	}

	s.rdb.Set(ctx, "user:status:"+userID, role, accessTokenTTL)
}

func (s *Service) generateTokenPair(
	ctx context.Context,
	userID, role string,
) (*TokenPair, error) {
	issue := tokenIssue{
		UserID:           userID,
		Role:             role,
		SessionID:        uuid.NewString(),
		RefreshJti:       uuid.NewString(),
		RefreshExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	if err := s.createRefreshSession(ctx, issue); err != nil {
		return nil, err
	}
	return s.signTokenPair(issue)
}

func (s *Service) rotateTokenPair(
	ctx context.Context,
	userID, role, sessionID, previousRefreshJti string,
) (*TokenPair, error) {
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	issue := tokenIssue{
		UserID:           userID,
		Role:             role,
		SessionID:        sessionID,
		RefreshJti:       uuid.NewString(),
		RefreshExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	rotation := refreshSessionRotation{
		UserID:             userID,
		SessionID:          sessionID,
		PreviousRefreshJti: previousRefreshJti,
		RefreshJti:         issue.RefreshJti,
		ExpiresAt:          issue.RefreshExpiresAt,
	}
	if err := s.rotateRefreshSession(ctx, rotation); err != nil {
		return nil, err
	}
	return s.signTokenPair(issue)
}

func (s *Service) createRefreshSession(
	ctx context.Context,
	issue tokenIssue,
) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO auth_sessions (id, user_id, refresh_jti, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)
	`, issue.SessionID, issue.UserID, issue.RefreshJti, issue.RefreshExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create auth session: %w", err)
	}
	return nil
}

func (s *Service) rotateRefreshSession(
	ctx context.Context,
	rotation refreshSessionRotation,
) error {
	if s.db == nil {
		return nil
	}
	if rotation.SessionID == "" || rotation.PreviousRefreshJti == "" {
		return fmt.Errorf("invalid refresh session")
	}
	var sessionID string
	err := s.db.QueryRow(ctx, `
		UPDATE auth_sessions
		SET refresh_jti = $4,
		    rotated_at = now(),
		    last_used_at = now(),
		    expires_at = $5
		WHERE id = $1::uuid
		  AND user_id = $2::uuid
		  AND refresh_jti = $3
		  AND revoked_at IS NULL
		  AND expires_at > now()
		RETURNING id::text
	`,
		rotation.SessionID,
		rotation.UserID,
		rotation.PreviousRefreshJti,
		rotation.RefreshJti,
		rotation.ExpiresAt,
	).Scan(&sessionID)
	if err == nil {
		return nil
	}
	if err == pgx.ErrNoRows {
		return fmt.Errorf("invalid refresh session")
	}
	return fmt.Errorf("failed to rotate auth session: %w", err)
}

func (s *Service) signTokenPair(issue tokenIssue) (*TokenPair, error) {
	issuedAt := time.Now()
	accessJti := uuid.NewString()
	accessExp := issuedAt.Add(accessTokenTTL)
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  issue.UserID,
		"role": issue.Role,
		"type": "access",
		"sid":  issue.SessionID,
		"jti":  accessJti,
		"iat":  issuedAt.Unix(),
		"exp":  accessExp.Unix(),
	})
	accessStr, err := access.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  issue.UserID,
		"role": issue.Role,
		"type": "refresh",
		"sid":  issue.SessionID,
		"jti":  issue.RefreshJti,
		"iat":  issuedAt.Unix(),
		"exp":  issue.RefreshExpiresAt.Unix(),
	})
	refreshStr, err := refresh.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    accessTokenExpiresInSec,
	}, nil
}
