package assistant

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	DefaultBridgeURL = "ws://127.0.0.1:8765/ws"
	BridgeReadLimit  = 8 << 20
)

var errBridgeNotConfigured = errors.New("assistant bridge not configured")

type wsDialFunc func(context.Context, string, *websocket.DialOptions) (*websocket.Conn, *http.Response, error)

type NoopBridge struct{}

type WebSocketBridge struct {
	upstreamURL string
	dial        wsDialFunc
}

type proxyResult struct {
	source string
	err    error
}

func NewWebSocketBridge(upstreamURL string) Bridge {
	upstreamURL = strings.TrimSpace(upstreamURL)
	if upstreamURL == "" {
		upstreamURL = DefaultBridgeURL
	}

	return &WebSocketBridge{
		upstreamURL: upstreamURL,
		dial:        websocket.Dial,
	}
}

func (NoopBridge) Proxy(ctx context.Context, _ Session, clientConn *websocket.Conn) error {
	ioCtx, cancel := backgroundIOContext()
	defer cancel()

	_ = writeBridgeError(ioCtx, clientConn, "assistant_unavailable", "assistant bridge not configured")
	_ = clientConn.Close(websocket.StatusNormalClosure, "assistant unavailable")
	return errBridgeNotConfigured
}

func (b *WebSocketBridge) Proxy(ctx context.Context, session Session, clientConn *websocket.Conn) error {
	clientConn.SetReadLimit(BridgeReadLimit)

	upstreamConn, _, err := b.dial(ctx, b.upstreamURL, nil)
	if err != nil {
		ioCtx, cancel := backgroundIOContext()
		defer cancel()

		_ = writeBridgeError(ioCtx, clientConn, "assistant_unavailable", "assistant service unavailable")
		_ = clientConn.Close(websocket.StatusNormalClosure, "assistant unavailable")
		return fmt.Errorf("dial assistant bridge: %w", err)
	}
	defer upstreamConn.CloseNow()
	upstreamConn.SetReadLimit(BridgeReadLimit)

	if err := wsjson.Write(ctx, upstreamConn, sessionInitEnvelope(session)); err != nil {
		ioCtx, cancel := backgroundIOContext()
		defer cancel()

		_ = writeBridgeError(ioCtx, clientConn, "assistant_init_failed", "assistant initialization failed")
		_ = clientConn.Close(websocket.StatusNormalClosure, "assistant initialization failed")
		return fmt.Errorf("send assistant init frame: %w", err)
	}

	proxyCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan proxyResult, 2)
	go proxyFrames(proxyCtx, "client", clientConn, upstreamConn, results)
	go proxyFrames(proxyCtx, "upstream", upstreamConn, clientConn, results)

	result := <-results
	cancel()

	switch result.source {
	case "upstream":
		ioCtx, cancel := backgroundIOContext()
		defer cancel()

		_ = writeBridgeError(ioCtx, clientConn, "assistant_upstream_disconnected", upstreamDisconnectMessage(result.err))
		_ = clientConn.Close(websocket.StatusNormalClosure, "assistant disconnected")
		_ = drainProxyResult(results)
		return result.err
	case "client":
		_ = upstreamConn.Close(websocket.StatusNormalClosure, "client disconnected")
		_ = drainProxyResult(results)
		if isNormalClientDisconnect(result.err) {
			return nil
		}
		return result.err
	default:
		return result.err
	}
}

func proxyFrames(ctx context.Context, source string, src, dst *websocket.Conn, results chan<- proxyResult) {
	for {
		msgType, payload, err := src.Read(ctx)
		if err != nil {
			results <- proxyResult{source: source, err: err}
			return
		}

		if err := dst.Write(ctx, msgType, payload); err != nil {
			results <- proxyResult{source: source, err: err}
			return
		}
	}
}

func sessionInitEnvelope(session Session) map[string]any {
	payload := map[string]any{
		"session_id": session.SessionID,
		"user_id":    session.UserID,
		"role":       session.Role,
		"token_type": session.TokenType,
		"media_mode": session.MediaMode,
	}
	if session.Locale != "" {
		payload["locale"] = session.Locale
	}
	if session.Profile != nil {
		payload["profile"] = session.Profile
	}
	if session.WakeGreeting != nil {
		payload["wake_greeting"] = *session.WakeGreeting
	}

	return map[string]any{
		"type":    "session_init",
		"payload": payload,
	}
}

func drainProxyResult(results <-chan proxyResult) error {
	select {
	case result := <-results:
		if result.source == "client" && isNormalClientDisconnect(result.err) {
			return nil
		}
		return result.err
	case <-time.After(250 * time.Millisecond):
		return nil
	}
}

func writeBridgeError(ctx context.Context, conn *websocket.Conn, code, message string) error {
	return writeEnvelope(ctx, conn, ServerEnvelope{
		Type: "error",
		Payload: map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func backgroundIOContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func isNormalClientDisconnect(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}

	switch websocket.CloseStatus(err) {
	case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		return true
	default:
		return false
	}
}

func upstreamDisconnectMessage(err error) string {
	if err == nil {
		return "assistant service disconnected"
	}

	if status := websocket.CloseStatus(err); status != -1 {
		return fmt.Sprintf("assistant service disconnected (status %d)", status)
	}

	return "assistant service disconnected"
}
