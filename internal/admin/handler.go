package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	svc adminService
}

type adminService interface {
	SearchUsers(ctx context.Context, query string, page, limit int32) ([]UserResult, error)
	GetUserProfile(ctx context.Context, userID string) (*UserProfile, error)
	GetFamilyLinks(ctx context.Context, userID string) ([]FamilyLinkResult, error)
	GetUserSubscription(ctx context.Context, userID string) (*AdminSubscription, error)
	GetUserUsage(ctx context.Context, userID string, page, limit int32) ([]AdminUsageEntry, error)
	GetUserEntitlements(ctx context.Context, userID string) ([]AdminEntitlementResult, error)
	GetSubscriptionEvents(ctx context.Context, page, limit int32) ([]AdminSubEvent, error)
	GetAuditLogs(ctx context.Context, userID string, page, limit int32) ([]map[string]any, error)
	GetAnalytics(ctx context.Context) (AdminAnalytics, error)
	GetAICostSummary(ctx context.Context, days int32) (AICostSummary, error)
	GetUserAICostEvents(ctx context.Context, userID string, limit int32) ([]AICostEventResult, error)
	GetTrackingSummary(ctx context.Context, days int32) (TrackingSummary, error)
	GetUserTimeline(ctx context.Context, userID string, limit int32) ([]UserTimelineEvent, error)
	RecordAdminTrackingEvent(ctx context.Context, actorID string, input AdminTrackingEventInput) (TrackingEvent, error)
	ListBillingTransactions(ctx context.Context, page, limit int32) ([]AdminBillingTransaction, error)
	ListUserBillingTransactions(ctx context.Context, userID string, page, limit int32) ([]AdminBillingTransaction, error)
}

func NewHandler(svc adminService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page := int32Param(r, "page", 1)
	limit := int32Param(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}

	users, err := h.svc.SearchUsers(r.Context(), q, page, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "search_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"users": users})
}

func (h *Handler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	profile, err := h.svc.GetUserProfile(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "profile_failed", err.Error())
		return
	}
	if profile == nil {
		response.Err(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}
	response.JSON(w, http.StatusOK, profile)
}

func (h *Handler) GetUserFamilyLinks(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	links, err := h.svc.GetFamilyLinks(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "family_links_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"family_links": links})
}

func (h *Handler) GetUserSubscription(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	sub, err := h.svc.GetUserSubscription(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "subscription_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, sub)
}

func (h *Handler) GetUserUsage(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	page := int32Param(r, "page", 1)
	limit := int32Param(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}

	usage, err := h.svc.GetUserUsage(r.Context(), userID, page, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "usage_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"entries": usage,
		"page":    page,
		"limit":   limit,
	})
}

func (h *Handler) GetUserEntitlements(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	entitlements, err := h.svc.GetUserEntitlements(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "entitlements_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"entitlements": entitlements})
}

func (h *Handler) ListBillingTransactions(w http.ResponseWriter, r *http.Request) {
	page := int32Param(r, "page", 1)
	limit := int32Param(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}

	transactions, err := h.svc.ListBillingTransactions(r.Context(), page, limit)
	if err != nil {
		response.Err(
			w,
			http.StatusInternalServerError,
			"billing_transactions_failed",
			err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"transactions": transactions,
		"page":         page,
		"limit":        limit,
	})
}

func (h *Handler) ListUserBillingTransactions(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	page := int32Param(r, "page", 1)
	limit := int32Param(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}

	transactions, err := h.svc.ListUserBillingTransactions(r.Context(), userID, page, limit)
	if err != nil {
		response.Err(
			w,
			http.StatusInternalServerError,
			"billing_transactions_failed",
			err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"transactions": transactions,
		"page":         page,
		"limit":        limit,
	})
}

func (h *Handler) ListSubscriptionEvents(w http.ResponseWriter, r *http.Request) {
	page := int32Param(r, "page", 1)
	limit := int32Param(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}

	events, err := h.svc.GetSubscriptionEvents(r.Context(), page, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "events_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"events": events,
		"page":   page,
		"limit":  limit,
	})
}

func (h *Handler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	page := int32Param(r, "page", 1)
	limit := int32Param(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}

	logs, err := h.svc.GetAuditLogs(r.Context(), userID, page, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "audit_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"logs":  logs,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.svc.GetAnalytics(r.Context())
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "analytics_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, analytics)
}

func (h *Handler) GetAICostSummary(w http.ResponseWriter, r *http.Request) {
	days := int32Param(r, "days", 30)
	summary, err := h.svc.GetAICostSummary(r.Context(), days)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "ai_cost_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, summary)
}

func (h *Handler) GetUserAICostEvents(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	limit := int32Param(r, "limit", 100)
	events, err := h.svc.GetUserAICostEvents(r.Context(), userID, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "ai_cost_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) GetTrackingSummary(w http.ResponseWriter, r *http.Request) {
	days := int32Param(r, "days", 30)
	if days > 365 {
		days = 365
	}
	summary, err := h.svc.GetTrackingSummary(r.Context(), days)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "tracking_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, summary)
}

func (h *Handler) GetUserTimeline(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	limit := int32Param(r, "limit", 100)
	if limit > 300 {
		limit = 300
	}
	events, err := h.svc.GetUserTimeline(r.Context(), userID, limit)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "timeline_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) RecordTrackingEvent(w http.ResponseWriter, r *http.Request) {
	var input AdminTrackingEventInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	event, err := h.svc.RecordAdminTrackingEvent(
		r.Context(),
		middleware.UserIDFromContext(r.Context()),
		input,
	)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "tracking_event_invalid", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, event)
}

type ManualGrantRequest struct {
	UsageType string  `json:"usage_type"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	Reason    string  `json:"reason"`
}

func (h *Handler) ManualGrant(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")

	var req ManualGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.UsageType == "" || req.Quantity <= 0 || req.Reason == "" {
		response.Err(w, http.StatusBadRequest, "validation", "usage_type, quantity (>0), and reason are required")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"status":     "granted",
		"user_id":    userID,
		"usage_type": req.UsageType,
		"quantity":   req.Quantity,
		"unit":       req.Unit,
		"reason":     req.Reason,
	})
}

func int32Param(r *http.Request, key string, fallback int32) int32 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v < 1 {
		return fallback
	}
	return int32(v)
}
