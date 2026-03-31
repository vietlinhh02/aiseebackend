package call

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
)

type Service struct {
	lk     *lk.Client
	pusher *push.Pusher
	hub    *chat.Hub
}

func NewService(
	lkClient *lk.Client, pusher *push.Pusher, hub *chat.Hub,
) *Service {
	return &Service{lk: lkClient, pusher: pusher, hub: hub}
}

func (s *Service) GetUserToken(
	ctx context.Context, userID, userName string,
) (string, error) {
	roomName := fmt.Sprintf("user-%s", userID)

	slog.Info("scheduling room creation and agent dispatch asynchronously",
		"room", roomName, "user", userID)

	// dispatch agent asynchronously to not block token generation
	go func() {
		// Use a background context since the request context will be canceled
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.lk.CreateRoomWithAgent(bgCtx, roomName); err != nil {
			slog.Error("agent dispatch failed",
				"room", roomName, "error", err)
		} else {
			slog.Info("agent dispatched successfully", "room", roomName)
		}
	}()

	return s.lk.CreateToken(roomName, userID, userName)
}

type CreateRoomResult struct {
	RoomName string `json:"room_name"`
	Token    string `json:"token"`
}

func (s *Service) CreateRoom(
	ctx context.Context,
	callerID, callerName string,
	callee dbgen.User,
	callType string,
) (CreateRoomResult, error) {
	shortID := uuid.New().String()[:8]
	roomName := fmt.Sprintf("call-%s", shortID)

	if err := s.lk.CreateRoom(ctx, roomName); err != nil {
		return CreateRoomResult{}, err
	}

	callerToken, err := s.lk.CreateToken(
		roomName, callerID, callerName,
	)
	if err != nil {
		return CreateRoomResult{},
			fmt.Errorf("create caller token: %w", err)
	}

	calleeToken, err := s.lk.CreateToken(
		roomName, pgutil.PgUUIDToString(callee.ID), callee.DisplayName,
	)
	if err != nil {
		return CreateRoomResult{},
			fmt.Errorf("create callee token: %w", err)
	}

	payload := map[string]string{
		"type":        "incoming_call",
		"caller_name": callerName,
		"caller_id":   callerID,
		"room_name":   roomName,
		"call_type":   callType,
		"token":       calleeToken,
	}

	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      callee.FcmToken,
		ApnsToken:     callee.ApnsToken,
		ApnsVoipToken: callee.ApnsVoipToken,
	}, push.Notification{
		Title: callerName,
		Body:  "Incoming call",
		Type:  "voip",
		Data:  payload,
	})

	calleeIDStr := pgutil.PgUUIDToString(callee.ID)
	if s.hub.IsOnline(calleeIDStr) {
		s.hub.SendToUser(calleeIDStr, chat.WSMessage{
			Type:    "incoming_call",
			Message: payload,
		})
	}

	return CreateRoomResult{
		RoomName: roomName, Token: callerToken,
	}, nil
}

func (s *Service) EndRoom(
	ctx context.Context, roomName string,
) error {
	return s.lk.DeleteRoom(ctx, roomName)
}
