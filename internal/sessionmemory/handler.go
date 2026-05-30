package sessionmemory

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// SaveRequest is the payload from the agent worker to persist session memory.
type SaveRequest struct {
	UserID       string   `json:"user_id"`
	SessionID    string   `json:"session_id"`
	Summary      string   `json:"summary"`
	RecentTurns  []Turn   `json:"recent_turns"`
	UserFacts    []string `json:"user_facts"`
	LastIntent   string   `json:"last_intent"`
	PendingTasks []string `json:"pending_tasks"`
	LastMode     string   `json:"last_mode"`
}

type Turn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// Save persists or updates session memory (upsert).
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	var req SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		return
	}
	if req.UserID == "" {
		response.Err(w, http.StatusBadRequest, "missing_field", "user_id is required")
		return
	}
	if req.SessionID == "" {
		req.SessionID = "default"
	}

	recentTurnsJSON, _ := json.Marshal(req.RecentTurns)
	userFactsJSON, _ := json.Marshal(req.UserFacts)
	pendingTasksJSON, _ := json.Marshal(req.PendingTasks)

	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO session_memories (user_id, session_id, summary, recent_turns, user_facts, last_intent, pending_tasks, last_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, session_id) DO UPDATE SET
			summary      = EXCLUDED.summary,
			recent_turns = EXCLUDED.recent_turns,
			user_facts   = EXCLUDED.user_facts,
			last_intent  = EXCLUDED.last_intent,
			pending_tasks = EXCLUDED.pending_tasks,
			last_mode    = EXCLUDED.last_mode
	`, req.UserID, req.SessionID, req.Summary,
		string(recentTurnsJSON), string(userFactsJSON),
		req.LastIntent, string(pendingTasksJSON), req.LastMode,
	)
	if err != nil {
		slog.Error("session_memory save failed", "error", err)
		response.Err(w, http.StatusInternalServerError, "db_error", "failed to save session memory")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Load returns the most recent session memory for a user.
func (h *Handler) Load(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	sessionID := r.URL.Query().Get("session_id")
	if userID == "" {
		response.Err(w, http.StatusBadRequest, "missing_field", "user_id is required")
		return
	}

	var (
		summary      string
		recentTurns  string
		userFacts    string
		lastIntent   string
		pendingTasks string
		lastMode     string
		fetchedSID   string
	)

	query := `
		SELECT session_id, summary, recent_turns::text, user_facts::text, last_intent, pending_tasks::text, last_mode
		FROM session_memories
		WHERE user_id = $1
	`
	args := []any{userID}

	if sessionID != "" {
		query += ` AND session_id = $2`
		args = append(args, sessionID)
	} else {
		query += ` ORDER BY updated_at DESC LIMIT 1`
	}

	err := h.pool.QueryRow(r.Context(), query, args...).Scan(
		&fetchedSID, &summary, &recentTurns, &userFacts, &lastIntent, &pendingTasks, &lastMode,
	)
	if err != nil {
		// No memory found — return empty (not an error)
		response.JSON(w, http.StatusOK, map[string]any{
			"found":    false,
			"summary":  "",
			"turns":    []any{},
			"facts":    []any{},
		})
		return
	}

	var turns []Turn
	var facts []string
	var tasks []string
	_ = json.Unmarshal([]byte(recentTurns), &turns)
	_ = json.Unmarshal([]byte(userFacts), &facts)
	_ = json.Unmarshal([]byte(pendingTasks), &tasks)

	response.JSON(w, http.StatusOK, map[string]any{
		"found":          true,
		"session_id":     fetchedSID,
		"summary":        summary,
		"turns":          turns,
		"facts":          facts,
		"last_intent":    lastIntent,
		"pending_tasks":  tasks,
		"last_mode":      lastMode,
	})
}

// Delete removes session memory for a specific session or all sessions for a user.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	sessionID := r.URL.Query().Get("session_id")
	if userID == "" {
		response.Err(w, http.StatusBadRequest, "missing_field", "user_id is required")
		return
	}

	if sessionID != "" {
		_, err := h.pool.Exec(r.Context(),
			`DELETE FROM session_memories WHERE user_id = $1 AND session_id = $2`,
			userID, sessionID,
		)
		if err != nil {
			response.Err(w, http.StatusInternalServerError, "db_error", "delete failed")
			return
		}
	} else {
		_, err := h.pool.Exec(r.Context(),
			`DELETE FROM session_memories WHERE user_id = $1`,
			userID,
		)
		if err != nil {
			response.Err(w, http.StatusInternalServerError, "db_error", "delete failed")
			return
		}
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
