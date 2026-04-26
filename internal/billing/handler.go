package billing

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type ReadService interface {
	ListPlans(context.Context) ([]Plan, error)
	CurrentSubscription(context.Context, string) (SubscriptionResponse, error)
	CurrentEntitlements(context.Context, string) (EntitlementsResponse, error)
	CurrentUsage(context.Context, string) (UsageResponse, error)
	RecordUsage(context.Context, UsageRecordInput) (UsageRecord, error)
	StartUsageSession(context.Context, UsageSessionInput) (UsageSession, error)
	CommitUsageSession(context.Context, CommitUsageSessionInput) (UsageRecord, error)
	GrantUsageQuota(context.Context, UsageGrantInput) (UsageGrant, error)
}

type Handler struct {
	service ReadService
}

func NewHandler(service ReadService) *Handler {
	return &Handler{service: service}
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
	record, err := h.service.RecordUsage(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "usage_invalid", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, record)
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
	response.JSON(w, http.StatusCreated, session)
}

func (h *Handler) CommitUsageSession(w http.ResponseWriter, r *http.Request) {
	var input CommitUsageSessionInput
	if !decodeBillingJSON(w, r, &input) {
		return
	}
	record, err := h.service.CommitUsageSession(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "usage_session_invalid", err.Error())
		return
	}
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
