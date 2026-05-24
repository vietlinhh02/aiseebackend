package care

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/tracking"
	"github.com/team-dang/aiseebackend/pkg/httpinput"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type careExtendedService interface {
	careService
	ListAlerts(ctx context.Context, requesterID, caredUserID, status string, limit int) ([]CareAlertRow, error)
	CaredUserIDForCareAlert(ctx context.Context, alertID string) (string, error)
	AckAlert(ctx context.Context, requesterID, caredUserID, alertID string) error
	ResolveAlert(ctx context.Context, requesterID, caredUserID, alertID string) error
	DismissAlert(ctx context.Context, requesterID, caredUserID, alertID string) error
	ListTasks(ctx context.Context, requesterID, caredUserID string, limit int) ([]CareTaskRow, error)
	CreateTask(ctx context.Context, requesterID, caredUserID string, in TaskCreateInput) (string, error)
	CaredUserIDForCareTask(ctx context.Context, taskID string) (string, error)
	UpdateTask(ctx context.Context, requesterID, caredUserID, taskID string, in TaskUpdateInput) error
	CompleteTask(ctx context.Context, requesterID, caredUserID, taskID string) error
	GetPermissions(ctx context.Context, requesterID, caredUserID string) (map[string]any, error)
	PutPermissions(ctx context.Context, requesterID, caredUserID, familyMemberID string, scopes map[string]any) error
	CoachQuery(ctx context.Context, requesterID, caredUserID, question string) (CoachQueryResponse, error)
	RunPatterns(ctx context.Context, requesterID, caredUserID string) (PatternRunResponse, error)
}

func (h *Handler) authorizeCared(
	w http.ResponseWriter,
	r *http.Request,
) (requesterID, caredUserID string, ok bool) {
	caredUserID = chi.URLParam(r, "user_id")
	if err := middleware.AuthorizeFamily(r.Context(), caredUserID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return "", "", false
	}
	return middleware.UserIDFromContext(r.Context()), caredUserID, true
}

func (h *Handler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	status := r.URL.Query().Get("status")
	limit := parseIntQuery(r, "limit", 50)
	rows, err := svc.ListAlerts(r.Context(), requesterID, caredID, status, limit)
	if err != nil {
		h.writeError(w, err, "alerts_failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"alerts": rows})
}

func (h *Handler) AckAlert(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	alertID := chi.URLParam(r, "alert_id")
	caredID, err := svc.CaredUserIDForCareAlert(r.Context(), alertID)
	if err != nil {
		h.writeError(w, err, "alert_ack_failed")
		return
	}
	if err := middleware.AuthorizeFamily(r.Context(), caredID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err := svc.AckAlert(r.Context(), requesterID, caredID, alertID); err != nil {
		h.writeError(w, err, "alert_ack_failed")
		return
	}
	h.recordSafetyEvent(r.Context(), "alert.acknowledged", caredID, requesterID, alertID, nil)
	response.JSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func (h *Handler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	alertID := chi.URLParam(r, "alert_id")
	caredID, err := svc.CaredUserIDForCareAlert(r.Context(), alertID)
	if err != nil {
		h.writeError(w, err, "alert_resolve_failed")
		return
	}
	if err := middleware.AuthorizeFamily(r.Context(), caredID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err := svc.ResolveAlert(r.Context(), requesterID, caredID, alertID); err != nil {
		h.writeError(w, err, "alert_resolve_failed")
		return
	}
	h.recordSafetyEvent(r.Context(), "alert.resolved", caredID, requesterID, alertID, nil)
	response.JSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (h *Handler) DismissAlert(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	alertID := chi.URLParam(r, "alert_id")
	caredID, err := svc.CaredUserIDForCareAlert(r.Context(), alertID)
	if err != nil {
		h.writeError(w, err, "alert_dismiss_failed")
		return
	}
	if err := middleware.AuthorizeFamily(r.Context(), caredID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err := svc.DismissAlert(r.Context(), requesterID, caredID, alertID); err != nil {
		h.writeError(w, err, "alert_dismiss_failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	limit := parseIntQuery(r, "limit", 100)
	rows, err := svc.ListTasks(r.Context(), requesterID, caredID, limit)
	if err != nil {
		h.writeError(w, err, "tasks_failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"tasks": rows})
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	var body struct {
		Title string         `json:"title" validate:"required"`
		DueAt *time.Time     `json:"due_at,omitempty"`
		Meta  map[string]any `json:"metadata,omitempty"`
	}
	if inputErr := httpinput.DecodeAndValidate(r, &body); inputErr != nil {
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "Title":
			response.Err(w, http.StatusBadRequest, "invalid_body", "title is required")
		default:
			response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		}
		return
	}
	id, err := svc.CreateTask(r.Context(), requesterID, caredID, TaskCreateInput{
		Title: body.Title,
		DueAt: body.DueAt,
		Meta:  body.Meta,
	})
	if err != nil {
		h.writeError(w, err, "task_create_failed")
		return
	}
	response.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	taskID := chi.URLParam(r, "task_id")
	caredID, err := svc.CaredUserIDForCareTask(r.Context(), taskID)
	if err != nil {
		h.writeError(w, err, "task_update_failed")
		return
	}
	if err := middleware.AuthorizeFamily(r.Context(), caredID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	var body struct {
		Title  string         `json:"title" validate:"required"`
		DueAt  *time.Time     `json:"due_at,omitempty"`
		Status string         `json:"status,omitempty"`
		Meta   map[string]any `json:"metadata,omitempty"`
	}
	if inputErr := httpinput.DecodeAndValidate(r, &body); inputErr != nil {
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "Title":
			response.Err(w, http.StatusBadRequest, "invalid_body", "title is required")
		default:
			response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		}
		return
	}
	if err := svc.UpdateTask(r.Context(), requesterID, caredID, taskID, TaskUpdateInput{
		Title: body.Title, DueAt: body.DueAt, Status: body.Status, Meta: body.Meta,
	}); err != nil {
		h.writeError(w, err, "task_update_failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	requesterID := middleware.UserIDFromContext(r.Context())
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	taskID := chi.URLParam(r, "task_id")
	caredID, err := svc.CaredUserIDForCareTask(r.Context(), taskID)
	if err != nil {
		h.writeError(w, err, "task_complete_failed")
		return
	}
	if err := middleware.AuthorizeFamily(r.Context(), caredID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err := svc.CompleteTask(r.Context(), requesterID, caredID, taskID); err != nil {
		h.writeError(w, err, "task_complete_failed")
		return
	}
	h.recordSafetyEvent(r.Context(), "task.completed", caredID, requesterID, taskID, nil)
	response.JSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *Handler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	scopes, err := svc.GetPermissions(r.Context(), requesterID, caredID)
	if err != nil {
		h.writeError(w, err, "permissions_failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"scopes": scopes})
}

func (h *Handler) PutPermissions(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	familyID := chi.URLParam(r, "family_id")
	var body map[string]any
	if inputErr := httpinput.DecodeAndValidate(r, &body); inputErr != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if err := svc.PutPermissions(r.Context(), requesterID, caredID, familyID, body); err != nil {
		h.writeError(w, err, "permissions_put_failed")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CoachQuery(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	var body struct {
		Question string `json:"question" validate:"required"`
	}
	if inputErr := httpinput.DecodeAndValidate(r, &body); inputErr != nil {
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "Question":
			response.Err(w, http.StatusBadRequest, "invalid_body", "question is required")
		default:
			response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		}
		return
	}
	resp, err := svc.CoachQuery(r.Context(), requesterID, caredID, body.Question)
	if err != nil {
		h.writeError(w, err, "coach_failed")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) RunPatterns(w http.ResponseWriter, r *http.Request) {
	requesterID, caredID, ok := h.authorizeCared(w, r)
	if !ok {
		return
	}
	svc, ok2 := h.svc.(careExtendedService)
	if !ok2 {
		response.Err(w, http.StatusInternalServerError, "service_misconfigured", "care service")
		return
	}
	resp, err := svc.RunPatterns(r.Context(), requesterID, caredID)
	if err != nil {
		h.writeError(w, err, "patterns_failed")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) recordSafetyEvent(ctx context.Context, eventType, caredID, actorID, sourceID string, meta map[string]any) {
	if h.tracking == nil {
		return
	}
	go func() {
		if meta == nil {
			meta = map[string]any{}
		}
		if _, err := h.tracking.RecordEvent(context.Background(), tracking.EventInput{
			EventType: eventType,
			Category:  "safety_care",
			UserID:    caredID,
			ActorID:   actorID,
			Source:    "care_handler",
			SourceID:  sourceID,
			Metadata:  meta,
		}); err != nil {
			slog.Error("record care tracking event", "event_type", eventType, "error", err)
		}
	}()
}
