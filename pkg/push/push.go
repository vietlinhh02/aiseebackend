package push

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
)

// UserTokens holds the push token fields needed for sending.
// Matches the token fields from dbgen.User without importing dbgen.
type UserTokens struct {
	FcmToken      pgtype.Text
	ApnsToken     pgtype.Text
	ApnsVoipToken pgtype.Text
}

type Notification struct {
	Title string
	Body  string
	Type  string            // "normal" | "voip"
	Data  map[string]string // custom payload
}

type Sender interface {
	Send(ctx context.Context, token string, n Notification) error
	Name() string
}

type Pusher struct {
	fcm  Sender
	apns Sender
}

func NewPusher(fcm, apns Sender) *Pusher {
	return &Pusher{fcm: fcm, apns: apns}
}

func (p *Pusher) Send(
	ctx context.Context,
	tokens UserTokens,
	n Notification,
) {
	if tokens.FcmToken.Valid && tokens.FcmToken.String != "" {
		if err := p.fcm.Send(
			ctx, tokens.FcmToken.String, n,
		); err != nil {
			slog.Error("fcm push failed", "error", err)
		}
	}

	if n.Type == "voip" &&
		tokens.ApnsVoipToken.Valid &&
		tokens.ApnsVoipToken.String != "" {
		if err := p.apns.Send(
			ctx, tokens.ApnsVoipToken.String, n,
		); err != nil {
			slog.Error("apns voip push failed", "error", err)
		}
	} else if tokens.ApnsToken.Valid &&
		tokens.ApnsToken.String != "" {
		if err := p.apns.Send(
			ctx, tokens.ApnsToken.String, n,
		); err != nil {
			slog.Error("apns push failed", "error", err)
		}
	}
}

// NoopSender is a no-op implementation of Sender for testing
// and environments where push is disabled.
type NoopSender struct{}

func (n *NoopSender) Send(
	_ context.Context, _ string, _ Notification,
) error {
	return nil
}

func (n *NoopSender) Name() string { return "noop" }
