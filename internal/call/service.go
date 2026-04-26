package call

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
)

type Service struct {
	lk          *lk.Client
	pusher      *push.Pusher
	hub         *chat.Hub
	activeCalls map[string]activeCall
	activeMu    sync.Mutex
}

func NewService(
	lkClient *lk.Client, pusher *push.Pusher, hub *chat.Hub,
) *Service {
	return &Service{
		lk:          lkClient,
		pusher:      pusher,
		hub:         hub,
		activeCalls: make(map[string]activeCall),
	}
}

type activeCall struct {
	CallerID     string
	CalleeID     string
	CallType     string
	CallerTokens push.UserTokens
	CalleeTokens push.UserTokens
}

func (s *Service) GetUserToken(
	ctx context.Context, userID, userName string,
) (string, error) {
	roomName := fmt.Sprintf("user-%s", userID)

	// Hoàn tất tạo phòng + dispatch agent trước khi trả JWT để app kết nối
	// khi worker đã nhận job (giảm chờ agent sau wake word).
	dispatchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	slog.Info("creating room and dispatching agent before token",
		"room", roomName, "user", userID)

	if err := s.lk.CreateRoomWithAgent(dispatchCtx, roomName); err != nil {
		slog.Error("agent dispatch failed",
			"room", roomName, "error", err)
		return "", fmt.Errorf("prepare voice room: %w", err)
	}

	slog.Info("agent dispatch ok, issuing token", "room", roomName)

	return s.lk.CreateToken(roomName, userID, userName)
}

type CreateRoomResult struct {
	RoomName string `json:"room_name"`
	Token    string `json:"token"`
}

func (s *Service) CreateRoom(
	ctx context.Context,
	caller dbgen.User,
	callee dbgen.User,
	callType string,
) (CreateRoomResult, error) {
	shortID := uuid.New().String()[:8]
	roomName := fmt.Sprintf("call-%s", shortID)
	callerID := pgutil.PgUUIDToString(caller.ID)
	callerName := caller.DisplayName
	calleeIDStr := pgutil.PgUUIDToString(callee.ID)

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
	if caller.AvatarUrl.Valid {
		payload["caller_avatar_url"] = caller.AvatarUrl.String
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

	s.rememberActiveCall(roomName, activeCall{
		CallerID: callerID,
		CalleeID: calleeIDStr,
		CallType: callType,
		CallerTokens: push.UserTokens{
			FcmToken:      caller.FcmToken,
			ApnsToken:     caller.ApnsToken,
			ApnsVoipToken: caller.ApnsVoipToken,
		},
		CalleeTokens: push.UserTokens{
			FcmToken:      callee.FcmToken,
			ApnsToken:     callee.ApnsToken,
			ApnsVoipToken: callee.ApnsVoipToken,
		},
	})
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
	ctx context.Context, requesterID, roomName string,
) error {
	if err := s.lk.DeleteRoom(ctx, roomName); err != nil {
		return err
	}
	if call, ok := s.forgetActiveCall(roomName); ok {
		s.notifyCallEnded(ctx, roomName, requesterID, call)
	}
	return nil
}

func (s *Service) rememberActiveCall(roomName string, call activeCall) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	s.activeCalls[roomName] = call
}

func (s *Service) forgetActiveCall(roomName string) (activeCall, bool) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	call, ok := s.activeCalls[roomName]
	if ok {
		delete(s.activeCalls, roomName)
	}
	return call, ok
}

func (s *Service) notifyCallEnded(
	ctx context.Context,
	roomName string,
	requesterID string,
	call activeCall,
) {
	recipientID := call.CalleeID
	if requesterID == call.CalleeID {
		recipientID = call.CallerID
	}
	if recipientID == "" || recipientID == requesterID {
		return
	}

	payload := map[string]string{
		"type":      "call_ended",
		"room_name": roomName,
		"ended_by":  requesterID,
		"call_type": call.CallType,
	}
	if s.hub.IsOnline(recipientID) {
		s.hub.SendToUser(recipientID, chat.WSMessage{
			Type:    "call_ended",
			Message: payload,
		})
	}
	if s.pusher == nil {
		return
	}
	tokens := call.CalleeTokens
	if recipientID == call.CallerID {
		tokens = call.CallerTokens
	}
	s.pusher.Send(ctx, tokens, push.Notification{
		Title: "AiSee",
		Body:  "Cuộc gọi đã kết thúc",
		Type:  "voip",
		Data:  payload,
	})
}
