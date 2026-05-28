package volunteer

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	lkproto "github.com/livekit/protocol/livekit"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/push"
)

type stubQuerier struct {
	registered bool
	available  bool
}

type stubUsageService struct {
	requireErr error
	records    []billing.UsageRecordInput
}

type stubRealtimeHub struct {
	onlineUserID  string
	onlineUserIDs map[string]bool
	messages      []chat.WSMessage
	sentTo        []string
}

func (s *stubRealtimeHub) IsOnline(userID string) bool {
	return userID == s.onlineUserID || s.onlineUserIDs[userID]
}

func (s *stubRealtimeHub) SendToUser(userID string, msg chat.WSMessage) {
	s.sentTo = append(s.sentTo, userID)
	s.messages = append(s.messages, msg)
}

type stubLiveKit struct {
	created      []string
	deleted      []string
	tokens       []string
	participants map[string][][]*lkproto.ParticipantInfo
}

func (s *stubLiveKit) CreateRoomWithTimeout(
	_ context.Context,
	roomName string,
	_ uint32,
) error {
	s.created = append(s.created, roomName)
	return nil
}

func (s *stubLiveKit) CreateToken(roomName, identity, _ string) (string, error) {
	token := "token:" + roomName + ":" + identity
	s.tokens = append(s.tokens, token)
	return token, nil
}

func (s *stubLiveKit) ListParticipants(
	_ context.Context,
	roomName string,
) ([]*lkproto.ParticipantInfo, error) {
	queues := s.participants[roomName]
	if len(queues) == 0 {
		return nil, nil
	}
	next := queues[0]
	s.participants[roomName] = queues[1:]
	return next, nil
}

func (s *stubLiveKit) DeleteRoom(_ context.Context, roomName string) error {
	s.deleted = append(s.deleted, roomName)
	return nil
}

func (s *stubUsageService) RequireQuota(
	_ context.Context,
	_ billing.QuotaCheckInput,
) error {
	return s.requireErr
}

func (s *stubUsageService) RecordUsage(
	_ context.Context,
	input billing.UsageRecordInput,
) (billing.UsageRecord, error) {
	s.records = append(s.records, input)
	return billing.UsageRecord{SourceID: input.SourceID}, nil
}

func (s *stubQuerier) CreateVolunteer(
	_ context.Context, _ dbgen.CreateVolunteerParams,
) (dbgen.Volunteer, error) {
	s.registered = true
	return dbgen.Volunteer{}, nil
}

func (s *stubQuerier) GetVolunteerByUserID(
	_ context.Context, _ pgtype.UUID,
) (dbgen.Volunteer, error) {
	return dbgen.Volunteer{}, nil
}

func (s *stubQuerier) UpdateVolunteerAvailability(
	_ context.Context, p dbgen.UpdateVolunteerAvailabilityParams,
) error {
	s.available = p.Available
	return nil
}

func (s *stubQuerier) GetAvailableVolunteers(
	_ context.Context, _ pgtype.UUID,
) ([]dbgen.GetAvailableVolunteersRow, error) {
	return nil, nil
}

func (s *stubQuerier) CreateVolunteerRating(
	_ context.Context, _ dbgen.CreateVolunteerRatingParams,
) error {
	return nil
}

func (s *stubQuerier) IncrementVolunteerCalls(
	_ context.Context, _ pgtype.UUID,
) error {
	return nil
}

func (s *stubQuerier) RecalcVolunteerRating(
	_ context.Context, _ pgtype.UUID,
) error {
	return nil
}

func (s *stubQuerier) GetUserByID(
	_ context.Context, _ pgtype.UUID,
) (dbgen.User, error) {
	return dbgen.User{}, nil
}

func TestRegister(t *testing.T) {
	store := &stubQuerier{}
	svc := NewService(store, nil, nil, nil)

	err := svc.Register(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		nil,
	)
	if err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if !store.registered {
		t.Fatal("Register: expected registered to be true")
	}
}

func TestSetAvailability(t *testing.T) {
	store := &stubQuerier{}
	svc := NewService(store, nil, nil, nil)

	err := svc.SetAvailability(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		true,
	)
	if err != nil {
		t.Fatalf(
			"SetAvailability: unexpected error: %v", err,
		)
	}
	if !store.available {
		t.Fatal(
			"SetAvailability: expected available to be true",
		)
	}
}

func TestStartAssistantMatchAllowsMissingBillingCareGroup(t *testing.T) {
	usage := &stubUsageService{
		requireErr: billing.ErrNoBillingCareGroup,
	}
	svc := NewService(&stubQuerier{}, nil, nil, nil, usage)

	result, err := svc.StartAssistantMatch(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		"voice",
	)
	if err != nil {
		t.Fatalf("StartAssistantMatch() error: %v", err)
	}
	if result.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", result.Status)
	}
	if len(usage.records) != 1 {
		t.Fatalf("RecordUsage calls = %d, want 1", len(usage.records))
	}
	if usage.records[0].UsageType != billing.UsageVolunteerMatches {
		t.Fatalf("usage type = %q", usage.records[0].UsageType)
	}
}

func TestSendVolunteerIncomingCallUsesWebSocketWhenVolunteerOnline(t *testing.T) {
	volunteerID := "22222222-2222-2222-2222-222222222222"
	hub := &stubRealtimeHub{onlineUserID: volunteerID}
	svc := NewService(
		&stubQuerier{},
		nil,
		nil,
		push.NewPusher(&push.NoopSender{}, &push.NoopSender{}),
	)
	svc.SetRealtimeHub(hub)

	svc.sendVolunteerIncomingCall(
		context.Background(),
		dbgen.User{
			ID:          pgUUID("11111111-1111-1111-1111-111111111111"),
			DisplayName: "Caller",
		},
		"11111111-1111-1111-1111-111111111111",
		dbgen.GetAvailableVolunteersRow{
			UserID:      pgUUID(volunteerID),
			DisplayName: "Volunteer",
		},
		"room-1",
		"token-1",
		"match-1",
		"voice",
	)

	if len(hub.messages) != 1 {
		t.Fatalf("websocket messages = %d, want 1", len(hub.messages))
	}
	if hub.messages[0].Type != "incoming_call" {
		t.Fatalf("message type = %q", hub.messages[0].Type)
	}
	payload, ok := hub.messages[0].Message.(map[string]string)
	if !ok {
		t.Fatalf("message payload type = %T", hub.messages[0].Message)
	}
	if payload["room_name"] != "room-1" || payload["token"] != "token-1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestWaitForJoinRequiresExpectedVolunteer(t *testing.T) {
	lk := &stubLiveKit{
		participants: map[string][][]*lkproto.ParticipantInfo{
			"room-1": {
				{{Identity: "33333333-3333-3333-3333-333333333333"}},
				{{Identity: "22222222-2222-2222-2222-222222222222"}},
				{{Identity: "22222222-2222-2222-2222-222222222222"}},
			},
		},
	}
	svc := NewService(&stubQuerier{}, nil, lk, nil)
	svc.answerPollInterval = time.Millisecond
	svc.answerJoinGrace = time.Millisecond
	svc.answerTimeout = 20 * time.Millisecond

	joined := svc.waitForJoin(
		context.Background(),
		"room-1",
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		svc.answerTimeout,
	)
	if !joined {
		t.Fatal("waitForJoin() = false, want true for expected volunteer")
	}
}

func TestWaitForJoinRejectsImmediateVolunteerHangup(t *testing.T) {
	lk := &stubLiveKit{
		participants: map[string][][]*lkproto.ParticipantInfo{
			"room-1": {
				{{Identity: "22222222-2222-2222-2222-222222222222"}},
				nil,
			},
		},
	}
	svc := NewService(&stubQuerier{}, nil, lk, nil)
	svc.answerPollInterval = time.Millisecond
	svc.answerJoinGrace = time.Millisecond
	svc.answerTimeout = 5 * time.Millisecond

	joined := svc.waitForJoin(
		context.Background(),
		"room-1",
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		svc.answerTimeout,
	)
	if joined {
		t.Fatal("waitForJoin() = true, want false after immediate hangup")
	}
}

func TestMatchCandidatesEndsVolunteerCallWhenNoAnswer(t *testing.T) {
	callerID := "11111111-1111-1111-1111-111111111111"
	volunteerID := "22222222-2222-2222-2222-222222222222"
	lk := &stubLiveKit{participants: map[string][][]*lkproto.ParticipantInfo{}}
	hub := &stubRealtimeHub{onlineUserID: volunteerID}
	svc := NewService(
		&stubQuerier{},
		nil,
		lk,
		push.NewPusher(&push.NoopSender{}, &push.NoopSender{}),
	)
	svc.SetRealtimeHub(hub)
	svc.answerPollInterval = time.Millisecond
	svc.answerJoinGrace = 0
	svc.answerTimeout = 3 * time.Millisecond

	result, err := svc.matchCandidates(
		context.Background(),
		callerID,
		dbgen.User{ID: pgUUID(callerID), DisplayName: "Caller"},
		[]dbgen.GetAvailableVolunteersRow{{
			UserID:      pgUUID(volunteerID),
			DisplayName: "Volunteer",
		}},
		"match-no-answer",
		"voice",
	)
	if err != nil {
		t.Fatalf("matchCandidates() error: %v", err)
	}
	if result.Status != "search_timeout" {
		t.Fatalf("status = %q, want search_timeout", result.Status)
	}
	if len(lk.deleted) != 1 || lk.deleted[0] != "vol-match-no-1" {
		t.Fatalf("deleted rooms = %#v", lk.deleted)
	}
	if len(hub.messages) != 2 {
		t.Fatalf("websocket messages = %d, want 2", len(hub.messages))
	}
	if hub.messages[0].Type != "incoming_call" || hub.messages[1].Type != "call_ended" {
		t.Fatalf("unexpected websocket messages: %#v", hub.messages)
	}
	payload, ok := hub.messages[1].Message.(map[string]string)
	if !ok {
		t.Fatalf("call_ended payload type = %T", hub.messages[1].Message)
	}
	if payload["reason"] != "no_answer" || payload["room_name"] != "vol-match-no-1" {
		t.Fatalf("unexpected call_ended payload: %#v", payload)
	}
}

func TestMatchCandidatesIgnoresLateWrongVolunteerAndMatchesExpected(t *testing.T) {
	callerID := "11111111-1111-1111-1111-111111111111"
	firstID := "22222222-2222-2222-2222-222222222222"
	secondID := "33333333-3333-3333-3333-333333333333"
	lk := &stubLiveKit{
		participants: map[string][][]*lkproto.ParticipantInfo{
			"vol-match-la-1": {
				{{Identity: secondID}},
			},
			"vol-match-la-2": {
				{{Identity: secondID}},
			},
		},
	}
	hub := &stubRealtimeHub{
		onlineUserIDs: map[string]bool{
			firstID:  true,
			secondID: true,
		},
	}
	svc := NewService(
		&stubQuerier{},
		nil,
		lk,
		push.NewPusher(&push.NoopSender{}, &push.NoopSender{}),
	)
	svc.SetRealtimeHub(hub)
	svc.answerPollInterval = time.Millisecond
	svc.answerJoinGrace = 0
	svc.answerTimeout = 3 * time.Millisecond

	result, err := svc.matchCandidates(
		context.Background(),
		callerID,
		dbgen.User{ID: pgUUID(callerID), DisplayName: "Caller"},
		[]dbgen.GetAvailableVolunteersRow{
			{UserID: pgUUID(firstID), DisplayName: "First"},
			{UserID: pgUUID(secondID), DisplayName: "Second"},
		},
		"match-late-wrong",
		"voice",
	)
	if err != nil {
		t.Fatalf("matchCandidates() error: %v", err)
	}
	if result.Status != "matched" || result.VolunteerID != secondID {
		t.Fatalf("unexpected match result: %+v", result)
	}
	if result.RoomName != "vol-match-la-2" {
		t.Fatalf("room = %q, want vol-match-la-2", result.RoomName)
	}
	if len(lk.deleted) != 1 || lk.deleted[0] != "vol-match-la-1" {
		t.Fatalf("deleted rooms = %#v", lk.deleted)
	}
}

func pgUUID(raw string) pgtype.UUID {
	return pgtype.UUID{
		Bytes: uuid.MustParse(raw),
		Valid: true,
	}
}
