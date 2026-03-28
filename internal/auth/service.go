package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type Service struct {
	db        *pgxpool.Pool
	queries   *dbgen.Queries
	jwtSecret string
}

func NewService(db *pgxpool.Pool, jwtSecret string) *Service {
	return &Service{
		db:        db,
		queries:   dbgen.New(db),
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
	}

	return s.generateTokenPair(user.ID.String(), user.Role)
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
	role, _ := claims["role"].(string)
	return s.generateTokenPair(sub, role)
}

func (s *Service) generateTokenPair(userID, role string) (*TokenPair, error) {
	accessExp := time.Now().Add(time.Hour)
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"type": "access",
		"exp":  accessExp.Unix(),
	})
	accessStr, err := access.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"type": "refresh",
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
