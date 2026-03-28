package push

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

type mockSender struct {
	name    string
	calls   []mockCall
	failErr error
}

type mockCall struct {
	token        string
	notification Notification
}

func (m *mockSender) Send(
	_ context.Context, token string, n Notification,
) error {
	m.calls = append(
		m.calls, mockCall{token: token, notification: n},
	)
	return m.failErr
}

func (m *mockSender) Name() string { return m.name }

func validText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func TestPusher_SendsFCM(t *testing.T) {
	fcm := &mockSender{name: "fcm"}
	apns := &mockSender{name: "apns"}
	p := NewPusher(fcm, apns)

	tokens := UserTokens{
		FcmToken: validText("fcm-token-123"),
	}
	n := Notification{
		Title: "Test", Body: "Hello", Type: "normal",
	}

	p.Send(context.Background(), tokens, n)

	if len(fcm.calls) != 1 {
		t.Fatalf(
			"expected 1 fcm call, got %d", len(fcm.calls),
		)
	}
	if fcm.calls[0].token != "fcm-token-123" {
		t.Errorf(
			"expected token 'fcm-token-123', got %q",
			fcm.calls[0].token,
		)
	}
	if len(apns.calls) != 0 {
		t.Errorf(
			"expected 0 apns calls, got %d",
			len(apns.calls),
		)
	}
}

func TestPusher_SendsAPNsStandard(t *testing.T) {
	fcm := &mockSender{name: "fcm"}
	apns := &mockSender{name: "apns"}
	p := NewPusher(fcm, apns)

	tokens := UserTokens{
		ApnsToken: validText("apns-token-456"),
	}
	n := Notification{
		Title: "Test", Body: "Hello", Type: "normal",
	}

	p.Send(context.Background(), tokens, n)

	if len(apns.calls) != 1 {
		t.Fatalf(
			"expected 1 apns call, got %d", len(apns.calls),
		)
	}
	if apns.calls[0].token != "apns-token-456" {
		t.Errorf(
			"expected token 'apns-token-456', got %q",
			apns.calls[0].token,
		)
	}
}

func TestPusher_SendsVoIPPush(t *testing.T) {
	fcm := &mockSender{name: "fcm"}
	apns := &mockSender{name: "apns"}
	p := NewPusher(fcm, apns)

	tokens := UserTokens{
		ApnsToken:     validText("apns-standard"),
		ApnsVoipToken: validText("apns-voip"),
	}
	n := Notification{Title: "Call", Type: "voip"}

	p.Send(context.Background(), tokens, n)

	if len(apns.calls) != 1 {
		t.Fatalf(
			"expected 1 apns call, got %d", len(apns.calls),
		)
	}
	if apns.calls[0].token != "apns-voip" {
		t.Errorf(
			"expected voip token, got %q",
			apns.calls[0].token,
		)
	}
}

func TestPusher_SendsBothFCMAndAPNs(t *testing.T) {
	fcm := &mockSender{name: "fcm"}
	apns := &mockSender{name: "apns"}
	p := NewPusher(fcm, apns)

	tokens := UserTokens{
		FcmToken:  validText("fcm-token"),
		ApnsToken: validText("apns-token"),
	}
	n := Notification{
		Title: "Test", Body: "Hello", Type: "normal",
	}

	p.Send(context.Background(), tokens, n)

	if len(fcm.calls) != 1 {
		t.Errorf(
			"expected 1 fcm call, got %d", len(fcm.calls),
		)
	}
	if len(apns.calls) != 1 {
		t.Errorf(
			"expected 1 apns call, got %d", len(apns.calls),
		)
	}
}

func TestPusher_SkipsEmptyTokens(t *testing.T) {
	fcm := &mockSender{name: "fcm"}
	apns := &mockSender{name: "apns"}
	p := NewPusher(fcm, apns)

	tokens := UserTokens{} // all empty
	n := Notification{Title: "Test", Type: "normal"}

	p.Send(context.Background(), tokens, n)

	if len(fcm.calls) != 0 {
		t.Errorf(
			"expected 0 fcm calls, got %d", len(fcm.calls),
		)
	}
	if len(apns.calls) != 0 {
		t.Errorf(
			"expected 0 apns calls, got %d",
			len(apns.calls),
		)
	}
}

func TestPusher_ContinuesOnFCMError(t *testing.T) {
	fcm := &mockSender{
		name: "fcm", failErr: errors.New("fcm down"),
	}
	apns := &mockSender{name: "apns"}
	p := NewPusher(fcm, apns)

	tokens := UserTokens{
		FcmToken:  validText("fcm-token"),
		ApnsToken: validText("apns-token"),
	}
	n := Notification{Title: "Test", Type: "normal"}

	p.Send(context.Background(), tokens, n)

	if len(apns.calls) != 1 {
		t.Errorf(
			"apns should still be called despite fcm error, got %d calls",
			len(apns.calls),
		)
	}
}
