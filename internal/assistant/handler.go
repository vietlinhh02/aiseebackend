package assistant

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Err(w, http.StatusUnauthorized, "missing_user", "authenticated user required")
		return
	}

	descriptor, err := h.service.CreateSession(
		userID,
		middleware.RoleFromContext(r.Context()),
		websocketURLFromRequest(r, "/api/v1/ai/ws"),
	)
	if err != nil {
		slog.Error("assistant_session_create_failed", "error", err, "user_id", userID)
		response.Err(w, http.StatusInternalServerError, "session_create_failed", "failed to create assistant session")
		return
	}

	response.JSON(w, http.StatusOK, descriptor)
}

func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		response.Err(w, http.StatusUnauthorized, "missing_token", "token query param required")
		return
	}

	session, err := h.service.ParseToken(tokenStr)
	if err != nil {
		response.Err(w, http.StatusUnauthorized, "invalid_token", "invalid assistant websocket token")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("assistant_ws_accept_failed", "error", err)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(BridgeReadLimit)

	ctx := r.Context()
	slog.Info("assistant_ws_connected", "user_id", session.UserID, "session_id", session.SessionID, "token_type", session.TokenType)
	if err := h.service.Proxy(ctx, session, conn); err != nil {
		slog.Warn("assistant_ws_proxy_failed", "error", err, "user_id", session.UserID, "session_id", session.SessionID)
	}

	slog.Info("assistant_ws_disconnected", "user_id", session.UserID, "session_id", session.SessionID)
}

func writeEnvelope(ctx context.Context, conn *websocket.Conn, envelope ServerEnvelope) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func websocketURLFromRequest(r *http.Request, path string) string {
	scheme := "ws"
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	switch proto {
	case "https", "wss":
		scheme = "wss"
	case "http", "ws":
		scheme = "ws"
	default:
		if r.TLS != nil {
			scheme = "wss"
		}
	}

	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}

	return scheme + "://" + host + path
}
