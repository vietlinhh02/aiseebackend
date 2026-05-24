package chat

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/user"
	"github.com/team-dang/aiseebackend/pkg/httpinput"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service     *Service
	hub         *Hub
	userService *user.Service
	jwtSecret   string
}

func NewHandler(
	service *Service,
	hub *Hub,
	userService *user.Service,
	jwtSecret string,
) *Handler {
	return &Handler{
		service:     service,
		hub:         hub,
		userService: userService,
		jwtSecret:   jwtSecret,
	}
}

type SendMessageRequest struct {
	ReceiverID  string `json:"receiver_id" validate:"required"`
	ContentText string `json:"content_text"`
	VoiceURL    string `json:"voice_url"`
	ImageURL    string `json:"image_url"`
}

func (h *Handler) SendMessage(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	var req SendMessageRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		if inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "ReceiverID" {
			response.Err(
				w, http.StatusBadRequest,
				"missing_receiver", "receiver_id is required",
			)
			return
		}
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}
	if req.ContentText == "" && req.VoiceURL == "" && req.ImageURL == "" {
		response.Err(
			w, http.StatusBadRequest,
			"empty_message",
			"content_text, voice_url or image_url required",
		)
		return
	}

	linked, err := h.userService.CheckFamilyLink(
		r.Context(), userID, req.ReceiverID,
	)
	if err != nil || !linked {
		response.Err(
			w, http.StatusForbidden,
			"not_family",
			"You can only message family members",
		)
		return
	}

	msg, err := h.service.SendMessage(
		r.Context(), userID, req.ReceiverID,
		req.ContentText, req.VoiceURL, req.ImageURL,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"send_failed", "Failed to send message",
		)
		return
	}
	response.JSON(w, http.StatusOK, msg)
}

func (h *Handler) GetMessages(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	contactID := chi.URLParam(r, "contactID")

	linked, err := h.userService.CheckFamilyLink(
		r.Context(), userID, contactID,
	)
	if err != nil || !linked {
		response.Err(
			w, http.StatusForbidden,
			"not_family",
			"You can only view family messages",
		)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := int32(50)
	if limitStr != "" {
		l, parseErr := strconv.Atoi(limitStr)
		if parseErr == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	beforeID := r.URL.Query().Get("before")

	var msgs []dbgen.Message
	if beforeID != "" {
		msgs, err = h.service.GetMessagesBefore(
			r.Context(), userID, contactID, beforeID, limit,
		)
	} else {
		msgs, err = h.service.GetMessages(
			r.Context(), userID, contactID, limit,
		)
	}
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"fetch_failed", "Failed to fetch messages",
		)
		return
	}
	response.JSON(
		w, http.StatusOK, map[string]any{"messages": msgs},
	)
}

func (h *Handler) MarkRead(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	messageID := chi.URLParam(r, "id")

	if err := h.service.MarkRead(
		r.Context(), messageID, userID,
	); err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"mark_failed", "Failed to mark message as read",
		)
		return
	}
	response.JSON(
		w, http.StatusOK, map[string]bool{"updated": true},
	)
}

func (h *Handler) UnreadCount(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	count, err := h.service.CountUnread(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"count_failed", "Failed to count unread",
		)
		return
	}
	response.JSON(
		w, http.StatusOK, map[string]int64{"count": count},
	)
}

func (h *Handler) WebSocket(
	w http.ResponseWriter, r *http.Request,
) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		response.Err(
			w, http.StatusUnauthorized,
			"missing_token", "token query param required",
		)
		return
	}

	token, err := jwt.Parse(
		tokenStr, func(t *jwt.Token) (any, error) {
			return []byte(h.jwtSecret), nil
		},
	)
	if err != nil || !token.Valid {
		response.Err(
			w, http.StatusUnauthorized,
			"invalid_token", "Invalid token",
		)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		response.Err(
			w, http.StatusUnauthorized,
			"invalid_claims", "Invalid claims",
		)
		return
	}
	userID, _ := claims["sub"].(string)

	conn, err := websocket.Accept(
		w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		},
	)
	if err != nil {
		slog.Error("websocket accept failed", "error", err)
		return
	}

	h.hub.Add(userID, conn)
	slog.Info("websocket connected", "user_id", userID)

	ctx := context.Background()
	for {
		_, _, readErr := conn.Read(ctx)
		if readErr != nil {
			break
		}
	}

	h.hub.Remove(userID)
	slog.Info("websocket disconnected", "user_id", userID)
}
