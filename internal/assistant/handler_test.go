package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
)

func TestCreateSessionReturnsProtocolDescriptor(t *testing.T) {
	t.Parallel()

	secret := "test-secret-at-least-32-characters"
	server := httptest.NewServer(newTestRouter(secret))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/ai/session", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, "user-123", "user", "access"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var descriptor SessionDescriptor
	if err := json.NewDecoder(resp.Body).Decode(&descriptor); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.HasPrefix(descriptor.SessionID, "ai-") {
		t.Fatalf("expected ai- session id, got %q", descriptor.SessionID)
	}
	if !strings.Contains(descriptor.WSURL, "/api/v1/ai/ws?token=") {
		t.Fatalf("expected ws_url to contain websocket token, got %q", descriptor.WSURL)
	}
	if descriptor.AudioInput.Encoding != "pcm_s16le" || descriptor.AudioInput.SampleRate != 16000 || descriptor.AudioInput.Channels != 1 {
		t.Fatalf("unexpected audio_input: %+v", descriptor.AudioInput)
	}
	if descriptor.AudioOutput.Encoding != "pcm_s16le" || descriptor.AudioOutput.SampleRate != 24000 || descriptor.AudioOutput.Channels != 1 {
		t.Fatalf("unexpected audio_output: %+v", descriptor.AudioOutput)
	}
	if descriptor.Camera.Encoding != "jpeg" || descriptor.Camera.MaxFPS != 1 {
		t.Fatalf("unexpected camera config: %+v", descriptor.Camera)
	}

	parsedURL, err := url.Parse(descriptor.WSURL)
	if err != nil {
		t.Fatalf("parse ws url: %v", err)
	}
	tokenStr := parsedURL.Query().Get("token")
	if tokenStr == "" {
		t.Fatal("expected session token in ws_url query")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("parse session token: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected map claims")
	}
	if got := claims["type"]; got != "assistant_session" {
		t.Fatalf("expected assistant_session token type, got %v", got)
	}
	if got := claims["sub"]; got != "user-123" {
		t.Fatalf("expected sub user-123, got %v", got)
	}
}

func TestWebSocketProxyBridgesJSONAndBinaryFrames(t *testing.T) {
	t.Parallel()

	secret := "test-secret-at-least-32-characters"
	upstreamURL, upstreamDone, initCh := newFakeAssistantUpstream(t, func(ctx context.Context, conn *websocket.Conn) error {
		msgType, payload, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read proxied text frame: %w", err)
		}
		if msgType != websocket.MessageText {
			return fmt.Errorf("expected text frame, got %v", msgType)
		}

		var envelope ClientEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode proxied text frame: %w", err)
		}
		if envelope.Type != "user_message" || envelope.RequestID != "req-text" {
			return fmt.Errorf("unexpected proxied text envelope: %+v", envelope)
		}

		if err := wsjson.Write(ctx, conn, ServerEnvelope{
			Type:      "assistant_reply",
			RequestID: envelope.RequestID,
			Payload: map[string]any{
				"text": "hello from python",
			},
		}); err != nil {
			return fmt.Errorf("write proxied text response: %w", err)
		}

		msgType, payload, err = conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read proxied binary frame: %w", err)
		}
		if msgType != websocket.MessageBinary {
			return fmt.Errorf("expected binary frame, got %v", msgType)
		}
		if !reflect.DeepEqual(payload, []byte{1, 2, 3, 4}) {
			return fmt.Errorf("unexpected proxied binary payload: %v", payload)
		}

		if err := conn.Write(ctx, websocket.MessageBinary, []byte{9, 8, 7}); err != nil {
			return fmt.Errorf("write proxied binary response: %w", err)
		}

		_, _, err = conn.Read(ctx)
		if status := websocket.CloseStatus(err); status == websocket.StatusNormalClosure {
			return nil
		}
		if err == nil {
			return nil
		}
		return fmt.Errorf("wait for downstream close: %w", err)
	})

	server := httptest.NewServer(newTestRouterWithBridge(secret, NewWebSocketBridge(upstreamURL)))
	defer server.Close()

	token := signedTokenWithClaims(t, secret, jwt.MapClaims{
		"sub":           "user-123",
		"role":          "user",
		"type":          "assistant_session",
		"scope":         "ai_ws",
		"sid":           "ai-session-123",
		"locale":        "vi-VN",
		"profile":       map[string]any{"display_name": "Dang"},
		"wake_greeting": true,
		"exp":           time.Now().Add(time.Hour).Unix(),
	})
	wsURL := websocketURL(server.URL, "/api/v1/ai/ws?token="+url.QueryEscape(token))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	if err := wsjson.Write(ctx, conn, map[string]any{
		"type":       "user_message",
		"request_id": "req-text",
		"payload":    map[string]any{"text": "hello"},
	}); err != nil {
		t.Fatalf("write text envelope: %v", err)
	}

	var reply ServerEnvelope
	if err := wsjson.Read(ctx, conn, &reply); err != nil {
		t.Fatalf("read proxied text response: %v", err)
	}
	if reply.Type != "assistant_reply" || reply.RequestID != "req-text" {
		t.Fatalf("unexpected proxied text response: %+v", reply)
	}

	if err := conn.Write(ctx, websocket.MessageBinary, []byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("write binary frame: %v", err)
	}

	msgType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read proxied binary response: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Fatalf("expected binary response, got %v", msgType)
	}
	if !reflect.DeepEqual(payload, []byte{9, 8, 7}) {
		t.Fatalf("unexpected proxied binary response: %v", payload)
	}

	initFrame := <-initCh
	if initFrame.Type != "session_init" {
		t.Fatalf("expected session_init frame, got %+v", initFrame)
	}
	if initFrame.Payload["session_id"] != "ai-session-123" {
		t.Fatalf("expected session_id in init frame, got %+v", initFrame.Payload)
	}
	if initFrame.Payload["user_id"] != "user-123" {
		t.Fatalf("expected user_id in init frame, got %+v", initFrame.Payload)
	}
	if initFrame.Payload["locale"] != "vi-VN" {
		t.Fatalf("expected locale in init frame, got %+v", initFrame.Payload)
	}
	if got := initFrame.Payload["wake_greeting"]; got != true {
		t.Fatalf("expected wake_greeting true, got %+v", initFrame.Payload)
	}
	profile, ok := initFrame.Payload["profile"].(map[string]any)
	if !ok || profile["display_name"] != "Dang" {
		t.Fatalf("expected profile payload, got %+v", initFrame.Payload["profile"])
	}

	if err := conn.Close(websocket.StatusNormalClosure, "test complete"); err != nil {
		t.Fatalf("close websocket: %v", err)
	}

	if err := <-upstreamDone; err != nil {
		t.Fatalf("upstream assertions failed: %v", err)
	}
}

func TestWebSocketProxyAcceptsCameraSizedBinaryFrames(t *testing.T) {
	t.Parallel()

	secret := "test-secret-at-least-32-characters"
	cameraFrame := make([]byte, 256*1024)
	cameraFrame[0] = 0xff
	cameraFrame[1] = 0xd8
	cameraFrame[2] = 0xff

	upstreamURL, upstreamDone, _ := newFakeAssistantUpstream(t, func(ctx context.Context, conn *websocket.Conn) error {
		msgType, payload, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read proxied camera frame: %w", err)
		}
		if msgType != websocket.MessageBinary {
			return fmt.Errorf("expected binary frame, got %v", msgType)
		}
		if len(payload) != len(cameraFrame) {
			return fmt.Errorf("expected %d camera bytes, got %d", len(cameraFrame), len(payload))
		}
		if err := conn.Write(ctx, websocket.MessageBinary, []byte{1}); err != nil {
			return fmt.Errorf("write camera ack: %w", err)
		}
		return nil
	})

	server := httptest.NewServer(newTestRouterWithBridge(secret, NewWebSocketBridge(upstreamURL)))
	defer server.Close()

	token := signedSessionToken(t, secret, "user-123", "user", "ai-session-camera")
	wsURL := websocketURL(server.URL, "/api/v1/ai/ws?token="+url.QueryEscape(token))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	if err := conn.Write(ctx, websocket.MessageBinary, cameraFrame); err != nil {
		t.Fatalf("write camera frame: %v", err)
	}

	msgType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read camera ack: %v", err)
	}
	if msgType != websocket.MessageBinary || !reflect.DeepEqual(payload, []byte{1}) {
		t.Fatalf("unexpected camera ack type=%v payload=%v", msgType, payload)
	}

	if err := <-upstreamDone; err != nil {
		t.Fatalf("upstream assertions failed: %v", err)
	}
}

func TestWebSocketProxySurfacesUpstreamDisconnect(t *testing.T) {
	t.Parallel()

	secret := "test-secret-at-least-32-characters"
	upstreamURL, upstreamDone, _ := newFakeAssistantUpstream(t, func(ctx context.Context, conn *websocket.Conn) error {
		return conn.Close(websocket.StatusGoingAway, "assistant offline")
	})

	server := httptest.NewServer(newTestRouterWithBridge(secret, NewWebSocketBridge(upstreamURL)))
	defer server.Close()

	token := signedSessionToken(t, secret, "user-123", "user", "ai-session-disconnect")
	wsURL := websocketURL(server.URL, "/api/v1/ai/ws?token="+url.QueryEscape(token))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.CloseNow()

	var envelope ServerEnvelope
	if err := wsjson.Read(ctx, conn, &envelope); err != nil {
		t.Fatalf("read upstream disconnect error envelope: %v", err)
	}
	if envelope.Type != "error" {
		t.Fatalf("expected error envelope, got %+v", envelope)
	}
	if got := envelope.Payload["code"]; got != "assistant_upstream_disconnected" {
		t.Fatalf("expected assistant_upstream_disconnected code, got %+v", envelope.Payload)
	}

	_, _, err = conn.Read(ctx)
	if status := websocket.CloseStatus(err); status != websocket.StatusNormalClosure && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("expected normal closure after error envelope, got status %v err %v", status, err)
	}

	if err := <-upstreamDone; err != nil {
		t.Fatalf("upstream assertions failed: %v", err)
	}
}

func TestParseTokenAcceptsAccessAndRejectsRefresh(t *testing.T) {
	t.Parallel()

	secret := "test-secret-at-least-32-characters"
	service := NewService(secret)

	accessSession, err := service.ParseToken(signedToken(t, secret, "user-123", "user", "access"))
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if accessSession.UserID != "user-123" || accessSession.TokenType != "access" {
		t.Fatalf("unexpected access session: %+v", accessSession)
	}
	if accessSession.MediaMode != MediaModeAudioPCM {
		t.Fatalf("expected default media mode %q, got %q", MediaModeAudioPCM, accessSession.MediaMode)
	}

	if _, err := service.ParseToken(signedToken(t, secret, "user-123", "user", "refresh")); err == nil {
		t.Fatal("expected refresh token to be rejected")
	}
}

func newTestRouter(secret string) http.Handler {
	return newTestRouterWithBridge(secret, NoopBridge{})
}

func newTestRouterWithBridge(secret string, bridge Bridge) http.Handler {
	service := NewServiceWithBridge(secret, bridge)
	handler := NewHandler(service)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(secret, nil, nil))
			r.Post("/ai/session", handler.CreateSession)
		})
		r.Get("/ai/ws", handler.WebSocket)
	})

	return r
}

func createSession(t *testing.T, baseURL, secret string) SessionDescriptor {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/ai/session", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, "user-123", "user", "access"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var descriptor SessionDescriptor
	if err := json.NewDecoder(resp.Body).Decode(&descriptor); err != nil {
		t.Fatalf("decode descriptor: %v", err)
	}
	return descriptor
}

func signedToken(t *testing.T, secret, sub, role, tokenType string) string {
	t.Helper()
	return signedTokenWithClaims(t, secret, jwt.MapClaims{
		"sub":  sub,
		"role": role,
		"type": tokenType,
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
}

func signedSessionToken(t *testing.T, secret, sub, role, sessionID string) string {
	t.Helper()
	return signedTokenWithClaims(t, secret, jwt.MapClaims{
		"sub":   sub,
		"role":  role,
		"type":  "assistant_session",
		"scope": "ai_ws",
		"sid":   sessionID,
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
}

func signedTokenWithClaims(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tokenStr
}

type initEnvelope struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

func newFakeAssistantUpstream(t *testing.T, session func(context.Context, *websocket.Conn) error) (string, <-chan error, <-chan initEnvelope) {
	t.Helper()

	done := make(chan error, 1)
	initCh := make(chan initEnvelope, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			done <- fmt.Errorf("accept upstream websocket: %w", err)
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(BridgeReadLimit)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var initMsg initEnvelope
		if err := wsjson.Read(ctx, conn, &initMsg); err != nil {
			done <- fmt.Errorf("read init frame: %w", err)
			return
		}
		initCh <- initMsg

		done <- session(ctx, conn)
	}))
	t.Cleanup(server.Close)

	return websocketURL(server.URL, "/"), done, initCh
}

func websocketURL(baseURL, path string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + path
}
