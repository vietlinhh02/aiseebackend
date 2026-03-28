package call

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/push"
)

type Service struct {
	lk     *lk.Client
	pusher *push.Pusher
}

func NewService(
	lkClient *lk.Client, pusher *push.Pusher,
) *Service {
	return &Service{lk: lkClient, pusher: pusher}
}

func (s *Service) GetUserToken(
	userID, userName string,
) (string, error) {
	roomName := fmt.Sprintf("user-%s", userID)
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

	token, err := s.lk.CreateToken(
		roomName, callerID, callerName,
	)
	if err != nil {
		return CreateRoomResult{},
			fmt.Errorf("create caller token: %w", err)
	}

	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      callee.FcmToken,
		ApnsToken:     callee.ApnsToken,
		ApnsVoipToken: callee.ApnsVoipToken,
	}, push.Notification{
		Title: callerName,
		Body:  "Incoming call",
		Type:  "voip",
		Data: map[string]string{
			"type":        "incoming_call",
			"caller_name": callerName,
			"caller_id":   callerID,
			"room_name":   roomName,
			"call_type":   callType,
		},
	})

	return CreateRoomResult{
		RoomName: roomName, Token: token,
	}, nil
}

func (s *Service) EndRoom(
	ctx context.Context, roomName string,
) error {
	return s.lk.DeleteRoom(ctx, roomName)
}
