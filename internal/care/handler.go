package care

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type careService interface {
	Timeline(
		ctx context.Context,
		requesterID string,
		userID string,
		opts TimelineOptions,
	) (TimelineResponse, error)
	DailyBrief(
		ctx context.Context,
		requesterID string,
		userID string,
		day time.Time,
	) (DailyBriefResponse, error)
	WeeklyReport(
		ctx context.Context,
		requesterID string,
		userID string,
		endDate time.Time,
	) (WeeklyReportResponse, error)
	IncidentReport(
		ctx context.Context,
		requesterID string,
		userID string,
	) (IncidentReportResponse, error)
	ExportPDF(
		ctx context.Context,
		requesterID string,
		userID string,
		req ExportPDFRequest,
	) (ExportResponse, error)
}

type Handler struct {
	svc     careService
	checker middleware.FamilyChecker
}

func NewHandler(
	svc careService,
	checker middleware.FamilyChecker,
) *Handler {
	return &Handler{svc: svc, checker: checker}
}

func (h *Handler) Timeline(
	w http.ResponseWriter,
	r *http.Request,
) {
	requesterID, userID, ok := h.authorize(w, r)
	if !ok {
		return
	}

	resp, err := h.svc.Timeline(r.Context(), requesterID, userID, TimelineOptions{
		Days:  parseIntQuery(r, "days", 7),
		Limit: parseIntQuery(r, "limit", 50),
	})
	if err != nil {
		h.writeError(w, err, "timeline_failed")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) DailyBrief(
	w http.ResponseWriter,
	r *http.Request,
) {
	requesterID, userID, ok := h.authorize(w, r)
	if !ok {
		return
	}

	var day time.Time
	raw := r.URL.Query().Get("date")
	if raw != "" {
		parsed, err := parseCareDate(raw, careLocation())
		if err != nil {
			response.Err(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		day = parsed
	}

	resp, err := h.svc.DailyBrief(r.Context(), requesterID, userID, day)
	if err != nil {
		h.writeError(w, err, "daily_brief_failed")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) WeeklyReport(
	w http.ResponseWriter,
	r *http.Request,
) {
	requesterID, userID, ok := h.authorize(w, r)
	if !ok {
		return
	}

	var endDate time.Time
	raw := r.URL.Query().Get("end_date")
	if raw != "" {
		parsed, err := parseCareDate(raw, careLocation())
		if err != nil {
			response.Err(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		endDate = parsed
	}

	resp, err := h.svc.WeeklyReport(r.Context(), requesterID, userID, endDate)
	if err != nil {
		h.writeError(w, err, "weekly_report_failed")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) IncidentReport(
	w http.ResponseWriter,
	r *http.Request,
) {
	requesterID, userID, ok := h.authorize(w, r)
	if !ok {
		return
	}

	resp, err := h.svc.IncidentReport(r.Context(), requesterID, userID)
	if err != nil {
		h.writeError(w, err, "incident_report_failed")
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) ExportPDF(
	w http.ResponseWriter,
	r *http.Request,
) {
	requesterID, userID, ok := h.authorize(w, r)
	if !ok {
		return
	}

	var req ExportPDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	resp, err := h.svc.ExportPDF(r.Context(), requesterID, userID, req)
	if err != nil {
		h.writeError(w, err, "pdf_export_failed")
		return
	}
	response.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) authorize(
	w http.ResponseWriter,
	r *http.Request,
) (string, string, bool) {
	userID := chi.URLParam(r, "user_id")
	if err := middleware.AuthorizeFamily(r.Context(), userID, h.checker); err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return "", "", false
	}
	return middleware.UserIDFromContext(r.Context()), userID, true
}

func (h *Handler) writeError(
	w http.ResponseWriter,
	err error,
	defaultCode string,
) {
	if apiErr, ok := AsAPIError(err); ok {
		response.Err(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}

	var quotaErr *billing.QuotaExceededError
	if errors.As(err, &quotaErr) {
		response.JSON(w, http.StatusPaymentRequired, map[string]any{
			"error": map[string]any{
				"code":         quotaErr.Code,
				"message":      "Quota exceeded",
				"usage_type":   quotaErr.UsageType,
				"required":     quotaErr.Required,
				"remaining":    quotaErr.Remaining,
				"plan_code":    quotaErr.PlanCode,
				"reset_period": quotaErr.ResetPeriod,
			},
		})
		return
	}

	response.Err(
		w,
		http.StatusInternalServerError,
		defaultCode,
		err.Error(),
	)
}

func parseIntQuery(
	r *http.Request,
	key string,
	fallback int,
) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
