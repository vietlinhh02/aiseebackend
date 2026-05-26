package volunteer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	lkproto "github.com/livekit/protocol/livekit"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/i18n"
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

type AssistantMatchResult struct {
	Status        string `json:"status"`
	MatchID       string `json:"match_id,omitempty"`
	VolunteerID   string `json:"volunteer_id,omitempty"`
	VolunteerName string `json:"volunteer_name,omitempty"`
	RoomName      string `json:"room_name,omitempty"`
	UserToken     string `json:"user_token,omitempty"`
	CallType      string `json:"call_type,omitempty"`
	Message       string `json:"message,omitempty"`
}

// Service handles volunteer registration, availability,
// matching, and rating.
type Service struct {
	q       Querier
	rdb     *redis.Client
	lk      liveKitClient
	pusher  *push.Pusher
	billing usageService
	hub     RealtimeHub

	answerTimeout      time.Duration
	answerPollInterval time.Duration
}

type RealtimeHub interface {
	IsOnline(userID string) bool
	SendToUser(userID string, msg chat.WSMessage)
}

type liveKitClient interface {
	CreateRoomWithTimeout(
		ctx context.Context,
		roomName string,
		emptyTimeoutSec uint32,
	) error
	CreateToken(roomName, identity, name string) (string, error)
	ListParticipants(
		ctx context.Context,
		roomName string,
	) ([]*lkproto.ParticipantInfo, error)
	DeleteRoom(ctx context.Context, roomName string) error
}

type usageService interface {
	RequireQuota(context.Context, billing.QuotaCheckInput) error
	RecordUsage(context.Context, billing.UsageRecordInput) (billing.UsageRecord, error)
}

// NewService creates a volunteer Service.
func NewService(
	q Querier,
	rdb *redis.Client,
	lkClient liveKitClient,
	pusher *push.Pusher,
	billingSvc ...usageService,
) *Service {
	var usage usageService
	if len(billingSvc) > 0 {
		usage = billingSvc[0]
	}
	return &Service{
		q:       q,
		rdb:     rdb,
		lk:      lkClient,
		pusher:  pusher,
		billing: usage,

		answerTimeout:      30 * time.Second,
		answerPollInterval: 5 * time.Second,
	}
}

func (s *Service) SetRealtimeHub(hub RealtimeHub) {
	s.hub = hub
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
	if s.billing != nil {
		if err := s.billing.RequireQuota(ctx, billing.QuotaCheckInput{
			UserID:    userID,
			UsageType: billing.UsageVolunteerMatches,
			Quantity:  1,
		}); err != nil && !errors.Is(err, billing.ErrNoBillingCareGroup) {
			return MatchResult{}, err
		}
	}

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

	if s.billing != nil {
		if _, err := s.billing.RecordUsage(ctx, billing.UsageRecordInput{
			UserID:    userID,
			UsageType: billing.UsageVolunteerMatches,
			Quantity:  1,
			Unit:      "matches",
			Source:    "volunteer_match",
			SourceID:  matchID,
			Metadata: map[string]any{
				"match_id": matchID,
			},
		}); err != nil {
			return MatchResult{}, fmt.Errorf("record volunteer usage: %w", err)
		}
	}

	go s.matchLoop(userID, matchID)

	return MatchResult{
		MatchID: matchID,
		Status:  "searching",
	}, nil
}

func (s *Service) StartAssistantMatch(
	ctx context.Context,
	userID string,
	callType string,
) (AssistantMatchResult, error) {
	matchID, err := s.createMatchUsage(ctx, userID)
	if err != nil {
		return AssistantMatchResult{}, err
	}

	user, candidates, err := s.loadMatchCandidates(ctx, userID)
	if err != nil {
		return AssistantMatchResult{}, err
	}

	result, err := s.matchCandidates(ctx, userID, user, candidates, matchID, normalizeCallType(callType))
	if err != nil {
		return AssistantMatchResult{}, err
	}
	return result, nil
}

func (s *Service) matchLoop(userID, matchID string) {
	ctx := context.Background()
	key := "match:" + matchID

	user, candidates, err := s.loadMatchCandidates(ctx, userID)
	if err != nil {
		slog.Error("matchLoop: load candidates", "error", err)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	result, err := s.matchCandidates(ctx, userID, user, candidates, matchID, "voice")
	if err != nil {
		slog.Error("matchLoop: match candidates", "error", err)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	fields := []any{
		"status", result.Status,
	}
	if result.VolunteerID != "" {
		fields = append(fields, "volunteer_id", result.VolunteerID)
	}
	if result.VolunteerName != "" {
		fields = append(fields, "volunteer_name", result.VolunteerName)
	}
	if result.RoomName != "" {
		fields = append(fields, "room_name", result.RoomName)
	}
	if result.UserToken != "" {
		fields = append(fields, "user_token", result.UserToken)
	}
	s.rdb.HSet(ctx, key, fields...)
}

func (s *Service) waitForJoin(
	ctx context.Context,
	roomName, callerID, expectedParticipantID string,
	timeout time.Duration,
) bool {
	pollInterval := s.answerPollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
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
				if p.Identity == expectedParticipantID &&
					p.Identity != callerID {
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

func (s *Service) createMatchUsage(
	ctx context.Context,
	userID string,
) (string, error) {
	if s.billing != nil {
		if err := s.billing.RequireQuota(ctx, billing.QuotaCheckInput{
			UserID:    userID,
			UsageType: billing.UsageVolunteerMatches,
			Quantity:  1,
		}); err != nil && !errors.Is(err, billing.ErrNoBillingCareGroup) {
			return "", err
		}
	}

	matchID := uuid.New().String()
	if s.billing != nil {
		if _, err := s.billing.RecordUsage(ctx, billing.UsageRecordInput{
			UserID:    userID,
			UsageType: billing.UsageVolunteerMatches,
			Quantity:  1,
			Unit:      "matches",
			Source:    "volunteer_match",
			SourceID:  matchID,
			Metadata: map[string]any{
				"match_id": matchID,
			},
		}); err != nil {
			return "", fmt.Errorf("record volunteer usage: %w", err)
		}
	}
	return matchID, nil
}

func (s *Service) loadMatchCandidates(
	ctx context.Context,
	userID string,
) (dbgen.User, []dbgen.GetAvailableVolunteersRow, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.User{}, nil, fmt.Errorf("parse user id: %w", err)
	}

	user, err := s.q.GetUserByID(ctx, uid)
	if err != nil {
		return dbgen.User{}, nil, fmt.Errorf("get user: %w", err)
	}

	candidates, err := s.q.GetAvailableVolunteers(ctx, uid)
	if err != nil {
		return dbgen.User{}, nil, fmt.Errorf("get available volunteers: %w", err)
	}
	return user, candidates, nil
}

func (s *Service) matchCandidates(
	ctx context.Context,
	userID string,
	user dbgen.User,
	candidates []dbgen.GetAvailableVolunteersRow,
	matchID string,
	callType string,
) (AssistantMatchResult, error) {
	if len(candidates) == 0 {
		return AssistantMatchResult{
			Status:   "unavailable",
			MatchID:  matchID,
			CallType: callType,
			Message:  "no volunteer is currently available",
		}, nil
	}

	for i, candidate := range candidates {
		volunteerID := pgutil.PgUUIDToString(candidate.UserID)
		roomName := volunteerRoomName(matchID, i)
		if err := s.lk.CreateRoomWithTimeout(ctx, roomName, 300); err != nil {
			return AssistantMatchResult{}, fmt.Errorf("create room: %w", err)
		}

		calleeToken, err := s.lk.CreateToken(roomName, volunteerID, candidate.DisplayName)
		if err != nil {
			slog.Error("matchCandidates: create volunteer token", "error", err, "volunteer_id", volunteerID)
			s.deleteVolunteerRoom(ctx, roomName)
			continue
		}

		s.sendVolunteerIncomingCall(ctx, user, userID, candidate, roomName, calleeToken, matchID, callType)
		if !s.waitForJoin(ctx, roomName, userID, volunteerID, s.answerTimeout) {
			s.notifyVolunteerCallEnded(ctx, candidate, roomName, userID, matchID, callType, "no_answer")
			s.deleteVolunteerRoom(ctx, roomName)
			continue
		}

		callerToken, err := s.lk.CreateToken(roomName, userID, user.DisplayName)
		if err != nil {
			slog.Error("matchCandidates: create caller token", "error", err)
			s.notifyVolunteerCallEnded(ctx, candidate, roomName, userID, matchID, callType, "connect_failed")
			s.deleteVolunteerRoom(ctx, roomName)
			continue
		}

		if err := s.q.IncrementVolunteerCalls(ctx, candidate.UserID); err != nil {
			slog.Error("matchCandidates: increment volunteer calls", "error", err)
		}

		return AssistantMatchResult{
			Status:        "matched",
			MatchID:       matchID,
			VolunteerID:   volunteerID,
			VolunteerName: candidate.DisplayName,
			RoomName:      roomName,
			UserToken:     callerToken,
			CallType:      callType,
		}, nil
	}

	return AssistantMatchResult{
		Status:   "search_timeout",
		MatchID:  matchID,
		CallType: callType,
		Message:  "no volunteer answered in time",
	}, nil
}

func (s *Service) sendVolunteerIncomingCall(
	ctx context.Context,
	user dbgen.User,
	userID string,
	candidate dbgen.GetAvailableVolunteersRow,
	roomName string,
	calleeToken string,
	matchID string,
	callType string,
) {
	payload := map[string]string{
		"type":        "incoming_call",
		"room_name":   roomName,
		"user_id":     userID,
		"caller_id":   userID,
		"caller_name": user.DisplayName,
		"call_type":   normalizeCallType(callType),
		"token":       calleeToken,
		"match_id":    matchID,
	}
	if user.AvatarUrl.Valid {
		payload["caller_avatar_url"] = user.AvatarUrl.String
	}

	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      candidate.FcmToken,
		ApnsToken:     candidate.ApnsToken,
		ApnsVoipToken: candidate.ApnsVoipToken,
	}, push.Notification{
		Title: "AiSee - " + user.DisplayName,
		Body:  i18n.T(i18n.FromUserSettings(candidate.Settings), "notif.volunteer.body"),
		Type:  "voip",
		Data:  payload,
	})

	volunteerID := pgutil.PgUUIDToString(candidate.UserID)
	if s.hub != nil && s.hub.IsOnline(volunteerID) {
		s.hub.SendToUser(volunteerID, chat.WSMessage{
			Type:    "incoming_call",
			Message: payload,
		})
	}
}

func (s *Service) notifyVolunteerCallEnded(
	ctx context.Context,
	candidate dbgen.GetAvailableVolunteersRow,
	roomName string,
	userID string,
	matchID string,
	callType string,
	reason string,
) {
	payload := map[string]string{
		"type":      "call_ended",
		"room_name": roomName,
		"ended_by":  userID,
		"call_type": normalizeCallType(callType),
		"match_id":  matchID,
		"reason":    reason,
	}

	volunteerID := pgutil.PgUUIDToString(candidate.UserID)
	if s.hub != nil && s.hub.IsOnline(volunteerID) {
		s.hub.SendToUser(volunteerID, chat.WSMessage{
			Type:    "call_ended",
			Message: payload,
		})
	}
	if s.pusher == nil {
		return
	}
	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      candidate.FcmToken,
		ApnsToken:     candidate.ApnsToken,
		ApnsVoipToken: candidate.ApnsVoipToken,
	}, push.Notification{
		Title: "AiSee",
		Body:  i18n.T(i18n.FromUserSettings(candidate.Settings), "notif.call_ended.body"),
		Type:  "voip",
		Data:  payload,
	})
}

func volunteerRoomName(matchID string, candidateIndex int) string {
	prefix := matchID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return fmt.Sprintf("vol-%s-%d", prefix, candidateIndex+1)
}

func (s *Service) deleteVolunteerRoom(ctx context.Context, roomName string) {
	if err := s.lk.DeleteRoom(ctx, roomName); err != nil {
		slog.Error("matchCandidates: delete room", "room", roomName, "error", err)
	}
}

func normalizeCallType(callType string) string {
	if callType == "video" {
		return "video"
	}
	return "voice"
}
