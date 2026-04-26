package volunteer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
)

// Querier defines the database methods the volunteer
// service depends on.
type Querier interface {
	CreateVolunteer(
		ctx context.Context,
		p dbgen.CreateVolunteerParams,
	) (dbgen.Volunteer, error)
	GetVolunteerByUserID(
		ctx context.Context, uid pgtype.UUID,
	) (dbgen.Volunteer, error)
	UpdateVolunteerAvailability(
		ctx context.Context,
		p dbgen.UpdateVolunteerAvailabilityParams,
	) error
	GetAvailableVolunteers(
		ctx context.Context, uid pgtype.UUID,
	) ([]dbgen.GetAvailableVolunteersRow, error)
	CreateVolunteerRating(
		ctx context.Context,
		p dbgen.CreateVolunteerRatingParams,
	) error
	IncrementVolunteerCalls(
		ctx context.Context, uid pgtype.UUID,
	) error
	RecalcVolunteerRating(
		ctx context.Context, uid pgtype.UUID,
	) error
	GetUserByID(
		ctx context.Context, uid pgtype.UUID,
	) (dbgen.User, error)
}

// MatchResult is returned immediately when a match
// search begins.
type MatchResult struct {
	MatchID string `json:"match_id"`
	Status  string `json:"status"`
}

// MatchStatus represents the current state of a
// volunteer match stored in Redis.
type MatchStatus struct {
	Status        string `json:"status"`
	VolunteerID   string `json:"volunteer_id,omitempty"`
	VolunteerName string `json:"volunteer_name,omitempty"`
	RoomName      string `json:"room_name,omitempty"`
	UserToken     string `json:"user_token,omitempty"`
}

// Service handles volunteer registration, availability,
// matching, and rating.
type Service struct {
	q      Querier
	rdb    *redis.Client
	lk     *lk.Client
	pusher *push.Pusher
}

// NewService creates a volunteer Service.
func NewService(
	q Querier,
	rdb *redis.Client,
	lkClient *lk.Client,
	pusher *push.Pusher,
) *Service {
	return &Service{
		q:      q,
		rdb:    rdb,
		lk:     lkClient,
		pusher: pusher,
	}
}

// Register creates a new volunteer profile for the user.
// Languages defaults to ["vi"] when empty.
func (s *Service) Register(
	ctx context.Context,
	userID string,
	languages []string,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	if len(languages) == 0 {
		languages = []string{"vi"}
	}

	_, err = s.q.CreateVolunteer(ctx, dbgen.CreateVolunteerParams{
		UserID:    uid,
		Languages: languages,
	})
	if err != nil {
		return fmt.Errorf("create volunteer: %w", err)
	}
	return nil
}

// GetRegistrationStatus checks if the user is a registered
// volunteer and returns their profile if they are.
func (s *Service) GetRegistrationStatus(
	ctx context.Context,
	userID string,
) (dbgen.Volunteer, bool, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Volunteer{}, false,
			fmt.Errorf("parse user id: %w", err)
	}

	vol, err := s.q.GetVolunteerByUserID(ctx, uid)
	if err != nil {
		// If using pgx, we should check for NoRows error if
		// sqlc doesn't handle it gracefully in the generated
		// code, but usually sqlc returns a zero value and
		// the error from the driver.
		return dbgen.Volunteer{}, false, nil
	}

	return vol, true, nil
}

// SetAvailability toggles the volunteer's availability.
func (s *Service) SetAvailability(
	ctx context.Context,
	userID string,
	available bool,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}

	err = s.q.UpdateVolunteerAvailability(
		ctx,
		dbgen.UpdateVolunteerAvailabilityParams{
			Available: available,
			UserID:    uid,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"update volunteer availability: %w", err,
		)
	}
	return nil
}

// StartMatch begins an asynchronous volunteer match.
// It returns immediately with a match ID that can be
// polled via GetMatchStatus.
func (s *Service) StartMatch(
	ctx context.Context, userID string,
) (MatchResult, error) {
	matchID := uuid.New().String()
	key := "match:" + matchID

	err := s.rdb.HSet(ctx, key,
		"status", "searching",
	).Err()
	if err != nil {
		return MatchResult{}, fmt.Errorf(
			"redis hset: %w", err,
		)
	}

	const matchTTL = 5 * time.Minute
	if err := s.rdb.Expire(ctx, key, matchTTL).Err(); err != nil {
		return MatchResult{}, fmt.Errorf(
			"redis expire: %w", err,
		)
	}

	go s.matchLoop(userID, matchID)

	return MatchResult{
		MatchID: matchID,
		Status:  "searching",
	}, nil
}

func (s *Service) matchLoop(userID, matchID string) {
	ctx := context.Background()
	key := "match:" + matchID

	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		slog.Error("matchLoop: parse user id",
			"error", err,
		)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	user, err := s.q.GetUserByID(ctx, uid)
	if err != nil {
		slog.Error("matchLoop: get user",
			"error", err,
		)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	candidates, err := s.q.GetAvailableVolunteers(ctx, uid)
	if err != nil {
		slog.Error("matchLoop: get available volunteers",
			"error", err,
		)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	if len(candidates) == 0 {
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	roomName := "vol-" + matchID[:8]
	if err := s.lk.CreateRoomWithTimeout(
		ctx, roomName, 300,
	); err != nil {
		slog.Error("matchLoop: create room",
			"room", roomName, "error", err,
		)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	for _, c := range candidates {
		volID := pgutil.PgUUIDToString(c.UserID)

		calleeToken, _ := s.lk.CreateToken(
			roomName, volID, c.DisplayName,
		)

		s.pusher.Send(ctx, push.UserTokens{
			FcmToken:      c.FcmToken,
			ApnsToken:     c.ApnsToken,
			ApnsVoipToken: c.ApnsVoipToken,
		}, push.Notification{
			Title: "AiSee - " + user.DisplayName,
			Body:  "Can giup do tu nguoi khuyet tat!",
			Type:  "voip",
			Data: map[string]string{
				"type":        "volunteer_match",
				"room_name":   roomName,
				"user_id":     userID,
				"caller_name": user.DisplayName,
				"token":       calleeToken,
				"match_id":    matchID,
			},
		})
		if s.waitForJoin(ctx, roomName, userID, 30*time.Second) {
			token, tokenErr := s.lk.CreateToken(
				roomName, userID, user.DisplayName,
			)
			if tokenErr != nil {
				slog.Error("matchLoop: create token",
					"error", tokenErr,
				)
				continue
			}

			s.rdb.HSet(ctx, key,
				"status", "matched",
				"volunteer_id", volID,
				"volunteer_name", c.DisplayName,
				"room_name", roomName,
				"user_token", token,
			)

			if incErr := s.q.IncrementVolunteerCalls(
				ctx, c.UserID,
			); incErr != nil {
				slog.Error(
					"matchLoop: increment calls",
					"error", incErr,
				)
			}
			return
		}
	}

	if err := s.lk.DeleteRoom(ctx, roomName); err != nil {
		slog.Error("matchLoop: delete room",
			"room", roomName, "error", err,
		)
	}
	s.rdb.HSet(ctx, key, "status", "failed")
}

func (s *Service) waitForJoin(
	ctx context.Context,
	roomName, callerID string,
	timeout time.Duration,
) bool {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline:
			return false
		case <-ticker.C:
			participants, err := s.lk.ListParticipants(
				ctx, roomName,
			)
			if err != nil {
				slog.Warn("waitForJoin: list participants",
					"room", roomName, "error", err,
				)
				continue
			}
			for _, p := range participants {
				if p.Identity != callerID &&
					p.Identity != "" {
					return true
				}
			}
		}
	}
}

// GetMatchStatus retrieves the current match status from
// Redis.
func (s *Service) GetMatchStatus(
	ctx context.Context, matchID string,
) (MatchStatus, error) {
	key := "match:" + matchID
	vals, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return MatchStatus{}, fmt.Errorf(
			"redis hgetall: %w", err,
		)
	}

	if len(vals) == 0 {
		return MatchStatus{Status: "not_found"}, nil
	}

	return MatchStatus{
		Status:        vals["status"],
		VolunteerID:   vals["volunteer_id"],
		VolunteerName: vals["volunteer_name"],
		RoomName:      vals["room_name"],
		UserToken:     vals["user_token"],
	}, nil
}

// Rate records a volunteer rating and recalculates
// the aggregate.
func (s *Service) Rate(
	ctx context.Context,
	volunteerID, raterID, matchID string,
	score int16,
) error {
	volUID, err := pgutil.ParsePgUUID(volunteerID)
	if err != nil {
		return fmt.Errorf("parse volunteer id: %w", err)
	}

	raterUID, err := pgutil.ParsePgUUID(raterID)
	if err != nil {
		return fmt.Errorf("parse rater id: %w", err)
	}

	err = s.q.CreateVolunteerRating(
		ctx,
		dbgen.CreateVolunteerRatingParams{
			VolunteerID: volUID,
			RaterID:     raterUID,
			MatchID:     matchID,
			Score:       score,
		},
	)
	if err != nil {
		return fmt.Errorf("create volunteer rating: %w", err)
	}

	if err := s.q.RecalcVolunteerRating(ctx, volUID); err != nil {
		return fmt.Errorf(
			"recalc volunteer rating: %w", err,
		)
	}
	return nil
}
