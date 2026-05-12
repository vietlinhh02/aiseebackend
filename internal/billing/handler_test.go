package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/team-dang/aiseebackend/internal/middleware"
)

type fakeReadService struct {
	plans              []Plan
	billingOverviewID  string
	billingOverview    BillingOverviewResponse
	subscriptionUserID string
	entitlementsUserID string
	subscription       SubscriptionResponse
	entitlements       EntitlementsResponse
	usageUserID        string
	usage              UsageResponse
	quotaInput         QuotaCheckInput
	quotaDecision      QuotaDecision
	recordInput        UsageRecordInput
	aiCostInput        AICostEventInput
	sessionInput       UsageSessionInput
	commitInput        CommitUsageSessionInput
	grantInput         UsageGrantInput
	revenueCatPayload  RevenueCatWebhookPayload
	revenueCatConfig   RevenueCatConfig
	revenueCatResult   RevenueCatSyncResult
}

func (f *fakeReadService) ListPlans(
	_ context.Context,
) ([]Plan, error) {
	return f.plans, nil
}

func (f *fakeReadService) CurrentSubscription(
	_ context.Context,
	userID string,
) (SubscriptionResponse, error) {
	f.subscriptionUserID = userID
	return f.subscription, nil
}

func (f *fakeReadService) CurrentBillingOverview(
	_ context.Context,
	userID string,
) (BillingOverviewResponse, error) {
	f.billingOverviewID = userID
	return f.billingOverview, nil
}

func (f *fakeReadService) CurrentEntitlements(
	_ context.Context,
	userID string,
) (EntitlementsResponse, error) {
	f.entitlementsUserID = userID
	return f.entitlements, nil
}

func (f *fakeReadService) CurrentUsage(
	_ context.Context,
	userID string,
) (UsageResponse, error) {
	f.usageUserID = userID
	return f.usage, nil
}

func (f *fakeReadService) CheckQuota(
	_ context.Context,
	input QuotaCheckInput,
) (QuotaDecision, error) {
	f.quotaInput = input
	if f.quotaDecision.UsageType == "" {
		f.quotaDecision = QuotaDecision{
			Allowed:   true,
			UsageType: input.UsageType,
			Required:  input.Quantity,
		}
	}
	return f.quotaDecision, nil
}

func (f *fakeReadService) RecordUsage(
	_ context.Context,
	input UsageRecordInput,
) (UsageRecord, error) {
	f.recordInput = input
	return UsageRecord{
		ID:        "usage-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
		Unit:      input.Unit,
		Source:    input.Source,
		SourceID:  input.SourceID,
	}, nil
}

func (f *fakeReadService) RecordBillableUsage(
	ctx context.Context,
	input UsageRecordInput,
) (UsageRecord, error) {
	return f.RecordUsage(ctx, input)
}

func (f *fakeReadService) RecordAICostEvent(
	_ context.Context,
	input AICostEventInput,
) (AICostEvent, error) {
	f.aiCostInput = input
	return AICostEvent{
		ID:         "ai-cost-1",
		UserID:     input.UserID,
		Provider:   input.Provider,
		Model:      input.Model,
		Modality:   input.Modality,
		Source:     input.Source,
		SourceID:   input.SourceID,
		CostUSD:    input.CostUSD,
		CostVND:    input.CostVND,
		Currency:   input.Currency,
		Estimated:  input.Estimated,
		OccurredAt: input.OccurredAt,
	}, nil
}

func (f *fakeReadService) StartUsageSession(
	_ context.Context,
	input UsageSessionInput,
) (UsageSession, error) {
	f.sessionInput = input
	return UsageSession{
		ID:        "session-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Source:    input.Source,
		SourceID:  input.SourceID,
		Status:    "started",
		StartedAt: input.StartedAt,
	}, nil
}

func (f *fakeReadService) CommitUsageSession(
	_ context.Context,
	input CommitUsageSessionInput,
) (UsageRecord, error) {
	f.commitInput = input
	return UsageRecord{
		ID:        "usage-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
		Unit:      input.Unit,
		Source:    input.Source,
		SourceID:  input.SourceID,
	}, nil
}

func (f *fakeReadService) GrantUsageQuota(
	_ context.Context,
	input UsageGrantInput,
) (UsageGrant, error) {
	f.grantInput = input
	return UsageGrant{
		ID:        "grant-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
		Unit:      input.Unit,
		Source:    input.Source,
		SourceID:  input.SourceID,
		Reason:    input.Reason,
	}, nil
}

func (f *fakeReadService) SyncRevenueCatWebhookWithConfig(
	_ context.Context,
	payload RevenueCatWebhookPayload,
	cfg RevenueCatConfig,
) (RevenueCatSyncResult, error) {
	f.revenueCatPayload = payload
	f.revenueCatConfig = cfg
	if f.revenueCatResult.Event.ID == "" {
		f.revenueCatResult.Event.ID = "event-1"
	}
	return f.revenueCatResult, nil
}

func TestListPlansReturnsBackendDefinedPlanEntitlements(
	t *testing.T,
) {
	svc := &fakeReadService{
		plans: []Plan{
			{
				Code:          "basic_149k",
				Name:          "Care Basic",
				PriceVND:      149000,
				BillingPeriod: "month",
				Status:        "active",
				SortOrder:     10,
				Entitlements: []Entitlement{
					{
						FeatureKey:  "ai_minutes_per_month",
						LimitValue:  float64(150),
						LimitUnit:   "minutes",
						ResetPeriod: "month",
					},
				},
			},
		},
	}
	h := NewHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	rr := httptest.NewRecorder()
	h.ListPlans(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Plans []Plan `json:"plans"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(resp.Plans))
	}
	if resp.Plans[0].Code != "basic_149k" {
		t.Errorf("unexpected plan code %q", resp.Plans[0].Code)
	}
	if got := resp.Plans[0].Entitlements[0].FeatureKey; got != "ai_minutes_per_month" {
		t.Errorf("unexpected entitlement key %q", got)
	}
}

func TestCurrentBillingOverviewUsesAuthenticatedUserAndReturnsCareGroup(
	t *testing.T,
) {
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	svc := &fakeReadService{
		billingOverview: BillingOverviewResponse{
			CareGroup: CareGroupSummary{
				ID:               "group-1",
				Name:             "Gia dinh me",
				CurrentUserRole:  "billing_admin",
				CanManageBilling: true,
				CaredUserCount:   2,
				MemberCount:      3,
			},
			Subscription: &Subscription{
				ID:       "sub-1",
				PlanCode: "plus_399k",
				Status:   "active",
			},
			EffectivePlan: PlanSummary{
				Code: "plus_399k",
				Name: "Care Plus",
			},
			Entitlements: map[string]EntitlementValue{
				"weekly_report_enabled": {LimitValue: true},
			},
			Usage: UsageResponse{
				Plan: PlanSummary{Code: "plus_399k"},
				Quotas: map[UsageType]UsageQuota{
					UsageAIMinutes: {
						UsageType: UsageAIMinutes,
						Limit:     600,
						Used:      480,
						Remaining: 120,
					},
				},
			},
			StoreManagement: StoreManagementInfo{
				Provider:     "google_play",
				Instructions: "Quản lý gói trong Google Play.",
			},
			AsOf: now,
		},
	}
	h := NewHandler(svc)

	req := httptest.NewRequest(
		http.MethodGet, "/api/v1/me/billing-overview", nil,
	)
	req = req.WithContext(
		middleware.WithUserID(
			req.Context(),
			"00000000-0000-0000-0000-000000000456",
		),
	)
	rr := httptest.NewRecorder()
	h.CurrentBillingOverview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.billingOverviewID != "00000000-0000-0000-0000-000000000456" {
		t.Fatalf("handler used user id %q", svc.billingOverviewID)
	}

	var resp BillingOverviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CareGroup.ID != "group-1" || !resp.CareGroup.CanManageBilling {
		t.Fatalf("unexpected care group: %+v", resp.CareGroup)
	}
	if resp.EffectivePlan.Code != "plus_399k" {
		t.Fatalf("unexpected effective plan: %+v", resp.EffectivePlan)
	}
	if got := resp.Usage.Quotas[UsageAIMinutes].Remaining; got != 120 {
		t.Fatalf("unexpected remaining AI quota %.2f", got)
	}
}

func TestCurrentBillingOverviewOmitsRawSubscriptionEventPayload(
	t *testing.T,
) {
	svc := &fakeReadService{
		billingOverview: BillingOverviewResponse{
			CareGroup: CareGroupSummary{
				ID:              "group-1",
				Name:            "Gia dinh me",
				CurrentUserRole: "family",
			},
			EffectivePlan: PlanSummary{Code: "free", Name: "Free / Essential"},
			Entitlements:  map[string]EntitlementValue{},
			Usage: UsageResponse{
				Plan:   PlanSummary{Code: "free"},
				Quotas: map[UsageType]UsageQuota{},
			},
			Events: []SubscriptionEvent{
				{
					ID:        "event-1",
					EventType: "provider_webhook",
					Provider:  "google_play",
					Payload: map[string]any{
						"purchase_token": "secret-token",
					},
				},
			},
		},
	}
	h := NewHandler(svc)
	req := httptest.NewRequest(
		http.MethodGet, "/api/v1/me/billing-overview", nil,
	)
	req = req.WithContext(
		middleware.WithUserID(
			req.Context(),
			"00000000-0000-0000-0000-000000000456",
		),
	)
	rr := httptest.NewRecorder()

	h.CurrentBillingOverview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "purchase_token") || strings.Contains(body, "secret-token") {
		t.Fatalf("response leaked raw provider payload: %s", body)
	}
}

func TestCurrentSubscriptionUsesAuthenticatedUser(
	t *testing.T,
) {
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	svc := &fakeReadService{
		subscription: SubscriptionResponse{
			Subscription: &Subscription{
				ID:       "sub-1",
				PlanCode: "plus_399k",
				Status:   "active",
			},
			EffectivePlan: PlanSummary{
				Code: "plus_399k",
				Name: "Care Plus",
			},
			AsOf: now,
		},
	}
	h := NewHandler(svc)

	req := httptest.NewRequest(
		http.MethodGet, "/api/v1/me/subscription", nil,
	)
	req = req.WithContext(
		middleware.WithUserID(
			req.Context(),
			"00000000-0000-0000-0000-000000000123",
		),
	)
	rr := httptest.NewRecorder()
	h.CurrentSubscription(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.subscriptionUserID != "00000000-0000-0000-0000-000000000123" {
		t.Fatalf("handler used user id %q", svc.subscriptionUserID)
	}

	var resp SubscriptionResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Subscription == nil || resp.Subscription.PlanCode != "plus_399k" {
		t.Fatalf("unexpected subscription response: %+v", resp.Subscription)
	}
}

func TestCurrentEntitlementsReturnsFeatureMap(
	t *testing.T,
) {
	svc := &fakeReadService{
		entitlements: EntitlementsResponse{
			Plan: PlanSummary{Code: "basic_149k", Name: "Care Basic"},
			Entitlements: map[string]EntitlementValue{
				"daily_brief_enabled": {
					LimitValue: true,
				},
				"family_members_limit": {
					LimitValue: float64(5),
					LimitUnit:  "members",
				},
			},
		},
	}
	h := NewHandler(svc)

	req := httptest.NewRequest(
		http.MethodGet, "/api/v1/me/entitlements", nil,
	)
	req = req.WithContext(
		middleware.WithUserID(
			req.Context(),
			"00000000-0000-0000-0000-000000000456",
		),
	)
	rr := httptest.NewRecorder()
	h.CurrentEntitlements(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.entitlementsUserID != "00000000-0000-0000-0000-000000000456" {
		t.Fatalf("handler used user id %q", svc.entitlementsUserID)
	}

	var resp EntitlementsResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Plan.Code != "basic_149k" {
		t.Fatalf("unexpected effective plan %q", resp.Plan.Code)
	}
	if value, ok := resp.Entitlements["daily_brief_enabled"]; !ok || value.LimitValue != true {
		t.Fatalf("missing daily brief entitlement: %+v", resp.Entitlements)
	}
}

func TestCurrentUsageUsesAuthenticatedUser(t *testing.T) {
	svc := &fakeReadService{
		usage: UsageResponse{
			Plan: PlanSummary{Code: "basic_149k"},
			Quotas: map[UsageType]UsageQuota{
				UsageAIMinutes: {
					UsageType: UsageAIMinutes,
					Limit:     150,
					Used:      10,
					Remaining: 140,
				},
			},
		},
	}
	h := NewHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/usage", nil)
	req = req.WithContext(
		middleware.WithUserID(
			req.Context(),
			"00000000-0000-0000-0000-000000000456",
		),
	)
	rr := httptest.NewRecorder()
	h.CurrentUsage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.usageUserID != "00000000-0000-0000-0000-000000000456" {
		t.Fatalf("handler used user id %q", svc.usageUserID)
	}
}

func TestRecordUsageDecodesInternalPayload(t *testing.T) {
	svc := &fakeReadService{}
	h := NewHandler(svc)
	body := []byte(`{
		"user_id":"00000000-0000-0000-0000-000000000456",
		"usage_type":"camera_activations",
		"quantity":1,
		"unit":"activations",
		"source":"agent",
		"source_id":"tool-call-1"
	}`)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/internal/usage/record",
		bytes.NewReader(body),
	)
	rr := httptest.NewRecorder()
	h.RecordUsage(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.recordInput.UsageType != UsageCameraActivations ||
		svc.recordInput.SourceID != "tool-call-1" {
		t.Fatalf("unexpected record input: %+v", svc.recordInput)
	}
}

func TestRevenueCatWebhookRequiresConfiguredAuthorization(t *testing.T) {
	h := NewHandlerWithConfig(&fakeReadService{}, "secret-token", RevenueCatConfig{})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/webhooks/revenuecat",
		strings.NewReader(`{"event":{"id":"evt-1"}}`),
	)
	rr := httptest.NewRecorder()

	h.RevenueCatWebhook(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRevenueCatWebhookDecodesPayloadAndConfig(t *testing.T) {
	svc := &fakeReadService{}
	h := NewHandlerWithConfig(
		svc,
		"secret-token",
		RevenueCatConfig{
			ProductPlanMap: "rc_basic=basic_149k",
		},
	)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/webhooks/revenuecat",
		strings.NewReader(`{
			"event": {
				"id": "evt-1",
				"type": "INITIAL_PURCHASE",
				"app_user_id": "00000000-0000-0000-0000-000000000123",
				"product_id": "rc_basic"
			}
		}`),
	)
	req.Header.Set("Authorization", "secret-token")
	rr := httptest.NewRecorder()

	h.RevenueCatWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.revenueCatPayload.Event.ProductID != "rc_basic" {
		t.Fatalf("product id = %q", svc.revenueCatPayload.Event.ProductID)
	}
	if svc.revenueCatConfig.ProductPlanMap != "rc_basic=basic_149k" {
		t.Fatalf("config = %+v", svc.revenueCatConfig)
	}
}

func TestRevenueCatWebhookAcceptsBearerAuthorizationVariant(t *testing.T) {
	svc := &fakeReadService{}
	h := NewHandlerWithConfig(svc, "secret-token", RevenueCatConfig{})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/webhooks/revenuecat",
		strings.NewReader(`{
			"event": {
				"id": "evt-1",
				"type": "INITIAL_PURCHASE",
				"app_user_id": "00000000-0000-0000-0000-000000000123",
				"product_id": "aisee_basic_149k_monthly"
			}
		}`),
	)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()

	h.RevenueCatWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
