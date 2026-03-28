package toolcall

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/response"
)

type ToolCallRequest struct {
	Tool   string         `json:"tool"`
	UserID string         `json:"user_id"`
	Params map[string]any `json:"params"`
}

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) HandleToolCall(w http.ResponseWriter, r *http.Request) {
	var req ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	slog.Info("tool call received", "tool", req.Tool, "user_id", req.UserID)

	// Phase 0: all tools return stub response
	// Real implementations added in Phase 1-2
	response.JSON(w, http.StatusOK, map[string]any{
		"tool":    req.Tool,
		"status":  "not_implemented",
		"message": "Tool will be implemented in Phase 1-2",
	})
}
