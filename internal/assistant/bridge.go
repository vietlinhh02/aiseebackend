package assistant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	DefaultBridgeURL        = "ws://127.0.0.1:8765/ws"
	BridgeReadLimit         = 8 << 20
	maxUpstreamRetries      = 8
	upstreamBackoffBaseSec  = 1
	upstreamBackoffMaxSec   = 30
	upstreamBackoffJitterPct = 0.5
)

var errBridgeNotConfigured = errors.New("assistant bridge not configured")

type wsDialFunc func(context.Context, string, *websocket.DialOptions) (*websocket.Conn, *http.Response, error)

type NoopBridge struct{}

type ReconnectCallback func(attempt int, userID, sessionID string)

type WebSocketBridge struct {
	upstreamURL   string
	dial          wsDialFunc
	cameraBlocked atomic.Bool
	MaxRetries    int
	OnReconnect   ReconnectCallback
}

type proxyResult struct {
	source string
	err    error
}

func NewWebSocketBridge(upstreamURL string) *WebSocketBridge {
	upstreamURL = strings.TrimSpace(upstreamURL)
	if upstreamURL == "" {
		upstreamURL = DefaultBridgeURL
	}

	return &WebSocketBridge{
		upstreamURL: upstreamURL,
		dial:        websocket.Dial,
		MaxRetries:  maxUpstreamRetries,
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
	b.cameraBlocked.Store(false)

	var lastUpstreamErr error
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			delay := upstreamRetryDelay(attempt - 1)
			if attempt > b.MaxRetries {
				delay = 0
			}
			if delay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}

		upstreamConn, err := b.dialUpstream(ctx, session)
		if err != nil {
			lastUpstreamErr = err
			if attempt < b.MaxRetries {
				continue
			}
			ioCtx, ioCancel := backgroundIOContext()
			defer ioCancel()
			_ = writeBridgeError(ioCtx, clientConn, "assistant_unavailable", fmt.Sprintf("assistant service unavailable after %d retries", b.MaxRetries))
			_ = clientConn.Close(websocket.StatusNormalClosure, "assistant unavailable")
			return lastUpstreamErr
		}

		if attempt > 0 && b.OnReconnect != nil {
			b.OnReconnect(attempt, session.UserID, session.SessionID)
		}

		proxyCtx, cancel := context.WithCancel(ctx)
		results := make(chan proxyResult, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			b.proxyFrames(proxyCtx, "client", clientConn, upstreamConn, results)
		}()
		go func() {
			defer wg.Done()
			b.proxyFrames(proxyCtx, "upstream", upstreamConn, clientConn, results)
		}()

		result := <-results
		cancel()

		if result.source == "client" {
			_ = upstreamConn.Close(websocket.StatusNormalClosure, "client disconnected")
			_ = clientConn.Close(websocket.StatusNormalClosure, "client disconnected")
			wg.Wait()
			if isNormalClientDisconnect(result.err) {
				return nil
			}
			return result.err
		}

		lastUpstreamErr = result.err

		if attempt < b.MaxRetries {
			upstreamConn.CloseNow()
			wg.Wait()
			continue
		}

		// Final retry exhausted — surface upstream error to client,
		// then tear down everything. Use a fresh background context
		// because the request context is cancelled at this point.
		ioCtx, ioCancel := backgroundIOContext()
		defer ioCancel()
		if err := writeBridgeError(ioCtx, clientConn, "assistant_upstream_disconnected", upstreamDisconnectMessage(lastUpstreamErr)); err != nil {
			slog.Warn("bridge_error_write_failed", "error", err)
		}
		_ = clientConn.Close(websocket.StatusNormalClosure, "assistant disconnected")
		upstreamConn.CloseNow()
		wg.Wait()
		return lastUpstreamErr
	}
}

func (b *WebSocketBridge) dialUpstream(ctx context.Context, session Session) (*websocket.Conn, error) {
	upstreamConn, _, err := b.dial(ctx, b.upstreamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial assistant bridge: %w", err)
	}
	upstreamConn.SetReadLimit(BridgeReadLimit)

	if err := wsjson.Write(ctx, upstreamConn, sessionInitEnvelope(session)); err != nil {
		upstreamConn.CloseNow()
		return nil, fmt.Errorf("send assistant init frame: %w", err)
	}

	b.cameraBlocked.Store(false)
	return upstreamConn, nil
}

func upstreamRetryDelay(attempt int) time.Duration {
	base := 1 << attempt
	if base > upstreamBackoffMaxSec {
		base = upstreamBackoffMaxSec
	}
	baseDur := time.Duration(base) * time.Second
	jitter := time.Duration(float64(baseDur) * upstreamBackoffJitterPct * rand.Float64())

	return baseDur + jitter
}

func (b *WebSocketBridge) proxyFrames(ctx context.Context, source string, src, dst *websocket.Conn, results chan<- proxyResult) {
	for {
		msgType, payload, err := src.Read(ctx)
		if err != nil {
			results <- proxyResult{source: source, err: err}
			return
		}

		if source == "client" {
			if msgType == websocket.MessageBinary {
				if b.cameraBlocked.Load() && len(payload) >= 2 &&
					payload[0] == 0xFF && payload[1] == 0xD8 {
					continue
				}
			} else if msgType == websocket.MessageText {
				if bytes.Contains(payload, []byte(`"nsfw_detected"`)) {
					b.cameraBlocked.Store(true)
				}
				if bytes.Contains(payload, []byte(`"media_mode"`)) &&
					bytes.Contains(payload, []byte(`"camera_jpeg"`)) {
					b.cameraBlocked.Store(false)
				}
			}
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
