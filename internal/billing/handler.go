package billing

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/tracking"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type ReadService interface {
	ListPlans(context.Context) ([]Plan, error)
	CurrentBillingOverview(context.Context, string) (BillingOverviewResponse, error)
	CurrentSubscription(context.Context, string) (SubscriptionResponse, error)
	CurrentEntitlements(context.Context, string) (EntitlementsResponse, error)
	CurrentUsage(context.Context, string) (UsageResponse, error)
	CheckQuota(context.Context, QuotaCheckInput) (QuotaDecision, error)
	RecordUsage(context.Context, UsageRecordInput) (UsageRecord, error)
	RecordBillableUsage(context.Context, UsageRecordInput) (UsageRecord, error)
	RecordAICostEvent(context.Context, AICostEventInput) (AICostEvent, error)
	StartUsageSession(context.Context, UsageSessionInput) (UsageSession, error)
	CommitUsageSession(context.Context, CommitUsageSessionInput) (UsageRecord, error)
	GrantUsageQuota(context.Context, UsageGrantInput) (UsageGrant, error)
	SyncRevenueCatWebhookWithConfig(
		context.Context, RevenueCatWebhookPayload, RevenueCatConfig,
	) (RevenueCatSyncResult, error)
}

type Handler struct {
	service             ReadService
	revenueCatAuthToken string
	revenueCatConfig    RevenueCatConfig
	tracking            *tracking.Service
}

func NewHandler(service ReadService) *Handler {
	return &Handler{service: service}
}

func NewHandlerWithTracking(
	service ReadService,
	trackingService *tracking.Service,
) *Handler {
	return &Handler{service: service, tracking: trackingService}
}

func NewHandlerWithConfig(
	service ReadService,
	revenueCatAuthToken string,
	revenueCatConfig RevenueCatConfig,
) *Handler {
	return &Handler{
		service:             service,
		revenueCatAuthToken: revenueCatAuthToken,
		revenueCatConfig:    revenueCatConfig,
	}
}

func NewHandlerWithConfigAndTracking(
	service ReadService,
	revenueCatAuthToken string,
	revenueCatConfig RevenueCatConfig,
	trackingService *tracking.Service,
) *Handler {
	return &Handler{
		service:             service,
		revenueCatAuthToken: revenueCatAuthToken,
		revenueCatConfig:    revenueCatConfig,
		tracking:            trackingService,
	}
}

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.service.ListPlans(r.Context())
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"plans_failed", "Failed to fetch plans",
		)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"plans": plans,
	})
}

func (h *Handler) CurrentSubscription(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	resp, err := h.service.CurrentSubscription(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"subscription_failed",
			"Failed to fetch subscription",
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) CurrentBillingOverview(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	resp, err := h.service.CurrentBillingOverview(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"billing_overview_failed",
			"Failed to fetch billing overview",
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) CurrentEntitlements(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	resp, err := h.service.CurrentEntitlements(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"entitlements_failed",
			"Failed to fetch entitlements",
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) CurrentUsage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	resp, err := h.service.CurrentUsage(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"usage_failed",
			"Failed to fetch usage",
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	var input UsageRecordInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	record, err := h.service.RecordBillableUsage(r.Context(), input)
	if err != nil {
		if writeQuotaError(w, err) {
			h.trackBilling(r.Context(), input.UserID, "quota_check_denied", input.Source, input.SourceID, map[string]any{
				"usage_type": input.UsageType,
				"quantity":   input.Quantity,
				"unit":       input.Unit,
			})
			return
		}
		response.Err(w, http.StatusBadRequest, "usage_invalid", err.Error())
		return
	}
	h.trackBilling(r.Context(), record.UserID, "usage_recorded", record.Source, record.SourceID, map[string]any{
		"usage_type": record.UsageType,
		"quantity":   record.Quantity,
		"unit":       record.Unit,
	})
	response.JSON(w, http.StatusCreated, record)
}

func (h *Handler) RecordAICostEvent(w http.ResponseWriter, r *http.Request) {
	var input AICostEventInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	event, err := h.service.RecordAICostEvent(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "ai_cost_invalid", err.Error())
		return
	}
	h.trackBilling(r.Context(), event.UserID, "ai_cost_recorded", event.Source, event.SourceID, map[string]any{
		"provider":  event.Provider,
		"model":     event.Model,
		"cost_usd":  event.CostUSD,
		"estimated": event.Estimated,
	})
	response.JSON(w, http.StatusCreated, event)
}

func (h *Handler) CheckQuota(w http.ResponseWriter, r *http.Request) {
	var input QuotaCheckInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	decision, err := h.service.CheckQuota(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "quota_invalid", err.Error())
		return
	}
	if !decision.Allowed {
		h.trackBilling(r.Context(), input.UserID, "quota_check_denied", "quota", string(input.UsageType), map[string]any{
			"usage_type":   decision.UsageType,
			"required":     decision.Required,
			"remaining":    decision.Remaining,
			"plan_code":    decision.PlanCode,
			"reset_period": decision.ResetPeriod,
		})
		_ = writeQuotaError(w, &QuotaExceededError{
			Code:        "quota_exceeded",
			UsageType:   decision.UsageType,
			Required:    decision.Required,
			Remaining:   decision.Remaining,
			PlanCode:    decision.PlanCode,
			ResetPeriod: decision.ResetPeriod,
		})
		return
	}
	response.JSON(w, http.StatusOK, decision)
}

func (h *Handler) StartUsageSession(w http.ResponseWriter, r *http.Request) {
	var input UsageSessionInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	session, err := h.service.StartUsageSession(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "usage_session_invalid", err.Error())
		return
	}
	h.trackBilling(r.Context(), session.UserID, "usage_session_started", session.Source, session.SourceID, map[string]any{
		"usage_type":        session.UsageType,
		"reserved_quantity": session.ReservedQuantity,
		"unit":              session.Unit,
	})
	response.JSON(w, http.StatusCreated, session)
}

func (h *Handler) CommitUsageSession(w http.ResponseWriter, r *http.Request) {
	var input CommitUsageSessionInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	record, err := h.service.CommitUsageSession(r.Context(), input)
	if err != nil {
		if writeQuotaError(w, err) {
			h.trackBilling(r.Context(), input.UserID, "quota_check_denied", input.Source, input.SourceID, map[string]any{
				"usage_type": input.UsageType,
				"quantity":   input.Quantity,
				"unit":       input.Unit,
			})
			return
		}
		response.Err(w, http.StatusBadRequest, "usage_session_invalid", err.Error())
		return
	}
	h.trackBilling(r.Context(), record.UserID, "usage_session_committed", record.Source, record.SourceID, map[string]any{
		"usage_type": record.UsageType,
		"quantity":   record.Quantity,
		"unit":       record.Unit,
	})
	response.JSON(w, http.StatusOK, record)
}

func (h *Handler) GrantUsageQuota(w http.ResponseWriter, r *http.Request) {
	var input UsageGrantInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	grant, err := h.service.GrantUsageQuota(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "usage_grant_invalid", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, grant)
}

func (h *Handler) trackBilling(
	ctx context.Context,
	userID string,
	eventType string,
	source string,
	sourceID string,
	metadata map[string]any,
) {
	if h.tracking == nil || strings.TrimSpace(userID) == "" {
		return
	}
	if strings.TrimSpace(source) == "" {
		source = "billing"
	}
	if strings.TrimSpace(sourceID) == "" {
		sourceID = eventType
	}
	_, _ = h.tracking.RecordEvent(ctx, tracking.EventInput{
		UserID:    userID,
		EventType: eventType,
		Category:  "system",
		Source:    source,
		SourceID:  sourceID,
		Metadata:  metadata,
	})
}

func (h *Handler) RevenueCatWebhook(w http.ResponseWriter, r *http.Request) {
	if h.revenueCatAuthToken == "" {
		response.Err(
			w, http.StatusServiceUnavailable,
			"revenuecat_not_configured",
			"RevenueCat webhook is not configured",
		)
		return
	}
	got := strings.TrimSpace(r.Header.Get("Authorization"))
	if !revenueCatAuthorizationMatches(got, h.revenueCatAuthToken) {
		response.Err(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var payload RevenueCatWebhookPayload
	if !decodeBillingJSON(w, r, &payload) {
		return
	}
	result, err := h.service.SyncRevenueCatWebhookWithConfig(
		r.Context(), payload, h.revenueCatConfig,
	)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "revenuecat_invalid", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func revenueCatAuthorizationMatches(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	for _, candidate := range []string{want, "Bearer " + want} {
		if subtle.ConstantTimeCompare([]byte(got), []byte(candidate)) == 1 {
			return true
		}
	}
	if strings.HasPrefix(want, "Bearer ") {
		rawWant := strings.TrimSpace(strings.TrimPrefix(want, "Bearer "))
		if rawWant != "" &&
			subtle.ConstantTimeCompare([]byte(got), []byte(rawWant)) == 1 {
			return true
		}
	}
	return false
}

func decodeBillingJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return false
	}
	return true
}

func writeQuotaError(w http.ResponseWriter, err error) bool {
	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) {
		return false
	}
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
	return true
}
