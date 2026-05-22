package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/internal/dbgen"
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

func (s *Service) AuthenticateGoogle(ctx context.Context, idToken, role string) (*TokenPair, error) {
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

	return s.generateTokenPair(user.ID.String(), effectiveRole)
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

	// Atomically check and invalidate the old refresh token (rotation)
	jti, _ := claims["jti"].(string)
	if exp, ok := claims["exp"].(float64); ok {
		ttl := time.Until(time.Unix(int64(exp), 0))
		if ttl > 0 && s.rdb != nil {
			ok, err := s.rdb.SetNX(ctx, "revoked:"+jti, "1", ttl).Result()
			if err != nil {
				return nil, fmt.Errorf("redis error: %w", err)
			}
			if !ok {
				return nil, fmt.Errorf("token revoked")
			}
		}
	}

	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	var dbErr error
	role, dbErr = s.currentBaseRole(ctx, sub, role)
	if dbErr != nil {
		return nil, fmt.Errorf("failed to refresh role: %w", dbErr)
	}
	effectiveRole := s.resolveEffectiveRole(ctx, sub, role)

	return s.generateTokenPair(sub, effectiveRole)
}

func (s *Service) currentBaseRole(ctx context.Context, userID, fallback string) (string, error) {
	if s.db == nil {
		return fallback, nil
	}
	var role string
	if err := s.db.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		return "", err
	}
	return role, nil
}

func (s *Service) cacheUserStatus(ctx context.Context, userID, role string) {
	if s.rdb == nil {
		return
	}

	// Cache for 1 hour, matches access token duration
	s.rdb.Set(ctx, "user:status:"+userID, role, time.Hour)
}

func (s *Service) generateTokenPair(userID, role string) (*TokenPair, error) {
	jti := uuid.New().String()
	accessExp := time.Now().Add(time.Hour)
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"type": "access",
		"jti":  jti,
		"exp":  accessExp.Unix(),
	})
	accessStr, err := access.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	refreshJti := uuid.New().String()
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"type": "refresh",
		"jti":  refreshJti,
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	refreshStr, err := refresh.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    3600,
	}, nil
}
