package chat

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
)

type Service struct {
	queries *dbgen.Queries
	db      *pgxpool.Pool
	hub     *Hub
	pusher  *push.Pusher
}

func NewService(
	db *pgxpool.Pool, hub *Hub, pusher *push.Pusher,
) *Service {
	return &Service{
		queries: dbgen.New(db),
		db:      db,
		hub:     hub,
		pusher:  pusher,
	}
}

func (s *Service) SendMessage(
	ctx context.Context,
	senderID, receiverID, contentText, voiceURL string,
) (dbgen.Message, error) {
	sid, err := pgutil.ParsePgUUID(senderID)
	if err != nil {
		return dbgen.Message{}, err
	}
	rid, err := pgutil.ParsePgUUID(receiverID)
	if err != nil {
		return dbgen.Message{}, err
	}

	msg, err := s.queries.CreateMessage(
		ctx, dbgen.CreateMessageParams{
			SenderID:   sid,
			ReceiverID: rid,
			ContentText: pgtype.Text{
				String: contentText, Valid: contentText != "",
			},
			VoiceUrl: pgtype.Text{
				String: voiceURL, Valid: voiceURL != "",
			},
		},
	)
	if err != nil {
		return dbgen.Message{}, fmt.Errorf("create message: %w", err)
	}

	sender, sErr := s.queries.GetUserByID(ctx, sid)
	if sErr == nil {
		receiver, rErr := s.queries.GetUserByID(ctx, rid)
		if rErr == nil {
			body := contentText
			if len(body) > 100 {
				body = body[:100] + "..."
			}
			if body == "" {
				body = "Voice message"
			}

			s.pusher.Send(ctx, push.UserTokens{
				FcmToken:      receiver.FcmToken,
				ApnsToken:     receiver.ApnsToken,
				ApnsVoipToken: receiver.ApnsVoipToken,
			}, push.Notification{
				Title: sender.DisplayName,
				Body:  body,
				Type:  "normal",
				Data: map[string]string{
					"type":      "message",
					"sender_id": pgutil.PgUUIDToString(sender.ID),
				},
			})
		}
	}

	s.hub.SendToUser(receiverID, WSMessage{
		Type:    "new_message",
		Message: msg,
	})

	return msg, nil
}

func (s *Service) GetMessages(
	ctx context.Context,
	userID, contactID string,
	limit int32,
) ([]dbgen.Message, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}
	cid, err := pgutil.ParsePgUUID(contactID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetMessagesByContact(
		ctx, dbgen.GetMessagesByContactParams{
			SenderID:   uid,
			ReceiverID: cid,
			Limit:      limit,
		},
	)
}

func (s *Service) GetMessagesBefore(
	ctx context.Context,
	userID, contactID, beforeID string,
	limit int32,
) ([]dbgen.Message, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}
	cid, err := pgutil.ParsePgUUID(contactID)
	if err != nil {
		return nil, err
	}
	bid, err := pgutil.ParsePgUUID(beforeID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetMessagesByContactBefore(
		ctx, dbgen.GetMessagesByContactBeforeParams{
			SenderID:   uid,
			ReceiverID: cid,
			ID:         bid,
			Limit:      limit,
		},
	)
}

func (s *Service) GetUnreadMessages(
	ctx context.Context, receiverID string,
) ([]dbgen.Message, error) {
	rid, err := pgutil.ParsePgUUID(receiverID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetUnreadMessages(ctx, rid)
}

func (s *Service) GetUnreadByContact(
	ctx context.Context, receiverID, senderID string,
) ([]dbgen.Message, error) {
	rid, err := pgutil.ParsePgUUID(receiverID)
	if err != nil {
		return nil, err
	}
	sid, err := pgutil.ParsePgUUID(senderID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetUnreadMessagesByContact(
		ctx, dbgen.GetUnreadMessagesByContactParams{
			ReceiverID: rid,
			SenderID:   sid,
		},
	)
}

func (s *Service) MarkRead(
	ctx context.Context, messageID, receiverID string,
) error {
	mid, err := pgutil.ParsePgUUID(messageID)
	if err != nil {
		return err
	}
	rid, err := pgutil.ParsePgUUID(receiverID)
	if err != nil {
		return err
	}
	return s.queries.MarkMessageRead(
		ctx, dbgen.MarkMessageReadParams{
			ID:         mid,
			ReceiverID: rid,
		},
	)
}

func (s *Service) CountUnread(
	ctx context.Context, receiverID string,
) (int64, error) {
	rid, err := pgutil.ParsePgUUID(receiverID)
	if err != nil {
		return 0, err
	}
	return s.queries.CountUnreadMessages(ctx, rid)
}
