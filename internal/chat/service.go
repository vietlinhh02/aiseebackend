package chat

import (
	"context"
	"fmt"
	"time"

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
	senderID, receiverID, contentText, voiceURL, imageURL string,
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
			ImageUrl: pgtype.Text{
				String: imageURL, Valid: imageURL != "",
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
			if body == "" && voiceURL != "" {
				body = "Voice message"
			}
			if body == "" && imageURL != "" {
				body = "Image"
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

	wsPayload := messagePayload(msg)
	if sErr == nil {
		wsPayload["sender_name"] = sender.DisplayName
		if sender.AvatarUrl.Valid {
			wsPayload["sender_avatar_url"] = sender.AvatarUrl.String
		}
	}
	s.hub.SendToUser(receiverID, WSMessage{
		Type:    "new_message",
		Message: wsPayload,
	})

	return msg, nil
}

func messagePayload(msg dbgen.Message) map[string]any {
	payload := map[string]any{
		"id":           pgutil.PgUUIDToString(msg.ID),
		"sender_id":    pgutil.PgUUIDToString(msg.SenderID),
		"receiver_id":  pgutil.PgUUIDToString(msg.ReceiverID),
		"content_text": "",
		"voice_url":    "",
		"image_url":    "",
		"is_read":      false,
		"created_at":   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if msg.ContentText.Valid {
		payload["content_text"] = msg.ContentText.String
	}
	if msg.VoiceUrl.Valid {
		payload["voice_url"] = msg.VoiceUrl.String
	}
	if msg.ImageUrl.Valid {
		payload["image_url"] = msg.ImageUrl.String
	}
	if msg.IsRead.Valid {
		payload["is_read"] = msg.IsRead.Bool
	}
	if msg.CreatedAt.Valid {
		payload["created_at"] = msg.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return payload
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
