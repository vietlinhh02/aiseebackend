package toolcall

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/call"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/internal/user"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type ToolCallRequest struct {
	Tool   string         `json:"tool"`
	UserID string         `json:"user_id"`
	Params map[string]any `json:"params"`
}

type Handler struct {
	chat    *chat.Service
	call    *call.Service
	user    *user.Service
	queries *dbgen.Queries
}

func NewHandler(
	chatSvc *chat.Service,
	callSvc *call.Service,
	userSvc *user.Service,
	queries *dbgen.Queries,
) *Handler {
	return &Handler{
		chat:    chatSvc,
		call:    callSvc,
		user:    userSvc,
		queries: queries,
	}
}

func (h *Handler) HandleToolCall(
	w http.ResponseWriter, r *http.Request,
) {
	var req ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	slog.Info("tool call received",
		"tool", req.Tool, "user_id", req.UserID,
	)

	result, err := h.dispatch(r.Context(), req)

	// Audit log (best-effort)
	h.logAudit(r.Context(), req, result, err)

	if err != nil {
		response.Err(
			w, http.StatusBadRequest, "tool_error", err.Error(),
		)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) dispatch(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	switch req.Tool {
	case "make_voice_call":
		return h.handleMakeCall(ctx, req, "voice")
	case "make_video_call":
		return h.handleMakeCall(ctx, req, "video")
	case "send_chat_message":
		return h.handleSendMessage(ctx, req)
	case "read_unread_messages":
		return h.handleReadUnread(ctx, req)
	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Tool)
	}
}

func (h *Handler) handleMakeCall(
	ctx context.Context, req ToolCallRequest, callType string,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)
	if contactName == "" {
		return nil, fmt.Errorf("contact_name is required")
	}

	member, err := h.user.ResolveContact(
		ctx, req.UserID, contactName,
	)
	if err != nil {
		return nil, err
	}

	caller, err := h.user.GetMe(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get caller: %w", err)
	}

	result, err := h.call.CreateRoom(
		ctx, req.UserID, caller.DisplayName,
		member.User, callType,
	)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}

	return map[string]string{
		"status":      "room_created",
		"room_name":   result.RoomName,
		"callee_name": member.User.DisplayName,
	}, nil
}

func (h *Handler) handleSendMessage(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)
	message, _ := req.Params["message"].(string)
	if contactName == "" || message == "" {
		return nil, fmt.Errorf(
			"contact_name and message are required",
		)
	}

	member, err := h.user.ResolveContact(
		ctx, req.UserID, contactName,
	)
	if err != nil {
		return nil, err
	}

	receiverID := pgutil.PgUUIDToString(member.User.ID)
	msg, err := h.chat.SendMessage(
		ctx, req.UserID, receiverID, message, "",
	)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	return map[string]any{
		"status":        "sent",
		"message_id":    pgutil.PgUUIDToString(msg.ID),
		"receiver_name": member.User.DisplayName,
	}, nil
}

func (h *Handler) handleReadUnread(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)

	var msgs []dbgen.Message
	var err error

	if contactName != "" {
		member, resolveErr := h.user.ResolveContact(
			ctx, req.UserID, contactName,
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		senderID := pgutil.PgUUIDToString(member.User.ID)
		msgs, err = h.chat.GetUnreadByContact(
			ctx, req.UserID, senderID,
		)
	} else {
		msgs, err = h.chat.GetUnreadMessages(ctx, req.UserID)
	}

	if err != nil {
		return nil, fmt.Errorf("get unread: %w", err)
	}

	type msgInfo struct {
		SenderName string `json:"sender_name"`
		Content    string `json:"content"`
		SentAt     string `json:"sent_at"`
	}
	var results []msgInfo
	for _, m := range msgs {
		sender, _ := h.queries.GetUserByID(ctx, m.SenderID)
		results = append(results, msgInfo{
			SenderName: sender.DisplayName,
			Content:    m.ContentText.String,
			SentAt:     m.CreatedAt.Time.Format(
				"2006-01-02T15:04:05+07:00",
			),
		})
	}

	return map[string]any{
		"messages": results,
		"count":    len(results),
	}, nil
}

func (h *Handler) logAudit(
	ctx context.Context,
	req ToolCallRequest,
	result any,
	toolErr error,
) {
	uid, err := pgutil.ParsePgUUID(req.UserID)
	if err != nil {
		return
	}

	paramsJSON, _ := json.Marshal(req.Params)
	var resultStr string
	if toolErr != nil {
		resultStr = fmt.Sprintf("error: %s", toolErr.Error())
	} else {
		resultJSON, _ := json.Marshal(result)
		resultStr = string(resultJSON)
	}

	_ = h.queries.CreateAuditLog(ctx, dbgen.CreateAuditLogParams{
		UserID:   uid,
		ToolName: req.Tool,
		Params:   paramsJSON,
		Result:   pgtype.Text{String: resultStr, Valid: true},
	})
}
