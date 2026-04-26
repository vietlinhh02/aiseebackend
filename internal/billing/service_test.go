package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	plans        []Plan
	entitlements map[string][]Entitlement
	subscription *Subscription
	records      []UsageRecordInput
	sessions     []UsageSessionInput
	commits      []CommitUsageSessionInput
	grants       []UsageGrantInput
	usageTotals  []UsageTotal
	grantTotals  []UsageTotal
}

func (f *fakeStore) ListPlans(
	_ context.Context,
) ([]Plan, error) {
	return f.plans, nil
}

func (f *fakeStore) DefaultPlan(
	_ context.Context,
) (Plan, error) {
	for _, plan := range f.plans {
		if plan.Status == "active" {
			return plan, nil
		}
	}
	return Plan{}, errors.New("no active plan")
}

func (f *fakeStore) CurrentSubscription(
	_ context.Context,
	_ string,
) (*Subscription, error) {
	return f.subscription, nil
}

func (f *fakeStore) PlanByID(
	_ context.Context,
	planID string,
) (Plan, error) {
	for _, plan := range f.plans {
		if plan.ID == planID {
			return plan, nil
		}
	}
	return Plan{}, errors.New("plan not found")
}

func (f *fakeStore) EntitlementsForPlan(
	_ context.Context,
	planID string,
) ([]Entitlement, error) {
	return f.entitlements[planID], nil
}

func (f *fakeStore) RecordUsage(
	_ context.Context,
	input UsageRecordInput,
) (UsageRecord, error) {
	f.records = append(f.records, input)
	return UsageRecord{
		ID:        "usage-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
		Unit:      input.Unit,
		Source:    input.Source,
		SourceID:  input.SourceID,
		CreatedAt: time.Date(2026, 4, 26, 1, 2, 3, 0, time.UTC),
	}, nil
}

func (f *fakeStore) StartUsageSession(
	_ context.Context,
	input UsageSessionInput,
) (UsageSession, error) {
	f.sessions = append(f.sessions, input)
	return UsageSession{
		ID:               "session-1",
		UserID:           input.UserID,
		UsageType:        input.UsageType,
		ReservedQuantity: input.ReservedQuantity,
		Unit:             input.Unit,
		Source:           input.Source,
		SourceID:         input.SourceID,
		Status:           "started",
		StartedAt:        input.StartedAt,
	}, nil
}

func (f *fakeStore) CommitUsageSession(
	_ context.Context,
	input CommitUsageSessionInput,
) (UsageRecord, error) {
	f.commits = append(f.commits, input)
	return UsageRecord{
		ID:        "usage-session-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
		Unit:      input.Unit,
		Source:    input.Source,
		SourceID:  input.SourceID,
		StartedAt: &input.StartedAt,
		EndedAt:   &input.EndedAt,
		CreatedAt: input.EndedAt,
	}, nil
}

func (f *fakeStore) GrantUsageQuota(
	_ context.Context,
	input UsageGrantInput,
) (UsageGrant, error) {
	f.grants = append(f.grants, input)
	return UsageGrant{
		ID:        "grant-1",
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
		Unit:      input.Unit,
		Source:    input.Source,
		SourceID:  input.SourceID,
		Reason:    input.Reason,
		CreatedAt: time.Date(2026, 4, 26, 1, 2, 3, 0, time.UTC),
	}, nil
}

func (f *fakeStore) UsageTotals(
	_ context.Context,
	_ string,
	_, _ time.Time,
) ([]UsageTotal, error) {
	return f.usageTotals, nil
}

func (f *fakeStore) UsageGrantTotals(
	_ context.Context,
	_ string,
	_, _ time.Time,
) ([]UsageTotal, error) {
	return f.grantTotals, nil
}

func TestServiceListPlansAttachesEntitlements(t *testing.T) {
	store := &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-basic",
				Code:          "basic_149k",
				Name:          "Care Basic",
				PriceVND:      149000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		entitlements: map[string][]Entitlement{
			"plan-basic": {
				{
					FeatureKey:  "pdf_exports_per_month",
					LimitValue:  float64(4),
					ResetPeriod: "month",
				},
			},
		},
	}

	plans, err := NewService(store).ListPlans(context.Background())
	if err != nil {
		t.Fatalf("ListPlans() error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("want 1 plan, got %d", len(plans))
	}
	if got := len(plans[0].Entitlements); got != 1 {
		t.Fatalf("want 1 entitlement, got %d", got)
	}
}

func TestServiceCurrentEntitlementsFallsBackToDefaultPlan(
	t *testing.T,
) {
	store := &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-basic",
				Code:          "basic_149k",
				Name:          "Care Basic",
				PriceVND:      149000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		entitlements: map[string][]Entitlement{
			"plan-basic": {
				{
					FeatureKey: "family_members_limit",
					LimitValue: float64(2),
					LimitUnit:  "members",
				},
			},
		},
	}

	resp, err := NewService(store).CurrentEntitlements(
		context.Background(),
		"00000000-0000-0000-0000-000000000789",
	)
	if err != nil {
		t.Fatalf("CurrentEntitlements() error: %v", err)
	}
	if resp.Plan.Code != "basic_149k" {
		t.Fatalf("want fallback basic plan, got %q", resp.Plan.Code)
	}
	limit := resp.Entitlements["family_members_limit"]
	if limit.LimitValue != float64(2) {
		t.Fatalf("unexpected family limit: %+v", limit)
	}
}

func TestServiceCurrentSubscriptionUsesActiveSubscriptionPlan(
	t *testing.T,
) {
	store := &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-plus",
				Code:          "plus_399k",
				Name:          "Care Plus",
				PriceVND:      399000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		subscription: &Subscription{
			ID:       "sub-plus",
			PlanID:   "plan-plus",
			PlanCode: "plus_399k",
			Status:   "active",
		},
	}

	resp, err := NewService(store).CurrentSubscription(
		context.Background(),
		"00000000-0000-0000-0000-000000000789",
	)
	if err != nil {
		t.Fatalf("CurrentSubscription() error: %v", err)
	}
	if resp.Subscription == nil {
		t.Fatal("expected current subscription")
	}
	if resp.EffectivePlan.Code != "plus_399k" {
		t.Fatalf("want plus effective plan, got %q", resp.EffectivePlan.Code)
	}
}

func TestServiceRecordUsageRequiresTrustedSourceAndAllowedType(
	t *testing.T,
) {
	store := &fakeStore{}
	svc := NewService(store)

	record, err := svc.RecordUsage(context.Background(), UsageRecordInput{
		UserID:    "00000000-0000-0000-0000-000000000789",
		UsageType: UsageAIMinutes,
		Quantity:  12.5,
		Unit:      "minutes",
		Source:    "agent",
		SourceID:  "livekit-room-123",
	})
	if err != nil {
		t.Fatalf("RecordUsage() error: %v", err)
	}
	if record.UsageType != UsageAIMinutes || record.SourceID == "" {
		t.Fatalf("unexpected usage record: %+v", record)
	}
	if len(store.records) != 1 {
		t.Fatalf("want 1 stored record, got %d", len(store.records))
	}

	_, err = svc.RecordUsage(context.Background(), UsageRecordInput{
		UserID:    "00000000-0000-0000-0000-000000000789",
		UsageType: UsageAIMinutes,
		Quantity:  1,
		Unit:      "minutes",
		Source:    "agent",
	})
	if err == nil {
		t.Fatal("expected missing source_id to fail")
	}

	_, err = svc.RecordUsage(context.Background(), UsageRecordInput{
		UserID:    "00000000-0000-0000-0000-000000000789",
		UsageType: UsageType("client_reported_minutes"),
		Quantity:  1,
		Unit:      "minutes",
		Source:    "agent",
		SourceID:  "bad-type",
	})
	if err == nil {
		t.Fatal("expected unsupported usage type to fail")
	}
}

func TestServiceCommitUsageSessionCalculatesMinutes(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	start := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	end := start.Add(10*time.Minute + 20*time.Second)

	record, err := svc.CommitUsageSession(
		context.Background(),
		CommitUsageSessionInput{
			UserID:    "00000000-0000-0000-0000-000000000789",
			UsageType: UsageAIMinutes,
			Unit:      "minutes",
			Source:    "agent",
			SourceID:  "livekit-room-123",
			StartedAt: start,
			EndedAt:   end,
		},
	)
	if err != nil {
		t.Fatalf("CommitUsageSession() error: %v", err)
	}
	if record.Quantity != 10.34 {
		t.Fatalf("want 10.34 committed minutes, got %.2f", record.Quantity)
	}
	if len(store.commits) != 1 || store.commits[0].Quantity != 10.34 {
		t.Fatalf("unexpected commit input: %+v", store.commits)
	}
}

func TestServiceCurrentUsageSummaryCombinesQuotaUsageAndGrants(
	t *testing.T,
) {
	store := &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-basic",
				Code:          "basic_149k",
				Name:          "Care Basic",
				PriceVND:      149000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		entitlements: map[string][]Entitlement{
			"plan-basic": {
				{
					FeatureKey:  "ai_minutes_per_month",
					LimitValue:  float64(150),
					LimitUnit:   "minutes",
					ResetPeriod: "month",
				},
				{
					FeatureKey:  "camera_activations_per_day",
					LimitValue:  float64(3),
					LimitUnit:   "activations",
					ResetPeriod: "day",
				},
				{
					FeatureKey:  "video_calls_per_day",
					LimitValue:  float64(5),
					LimitUnit:   "calls",
					ResetPeriod: "day",
				},
			},
		},
		usageTotals: []UsageTotal{
			{
				UsageType: UsageAIMinutes,
				Quantity:  140,
				Unit:      "minutes",
			},
			{
				UsageType: UsageCameraActivations,
				Quantity:  2,
				Unit:      "activations",
			},
			{
				UsageType: UsageVideoCallMinutes,
				Quantity:  2,
				Unit:      "calls",
			},
		},
		grantTotals: []UsageTotal{
			{
				UsageType: UsageAIMinutes,
				Quantity:  15,
				Unit:      "minutes",
			},
		},
	}

	resp, err := NewService(store).CurrentUsage(
		context.Background(),
		"00000000-0000-0000-0000-000000000789",
	)
	if err != nil {
		t.Fatalf("CurrentUsage() error: %v", err)
	}

	ai := resp.Quotas[UsageAIMinutes]
	if ai.Limit != 150 || ai.Used != 140 || ai.Granted != 15 ||
		ai.Remaining != 25 {
		t.Fatalf("unexpected AI quota: %+v", ai)
	}
	if !ai.UpgradeSuggested {
		t.Fatal("expected AI quota to suggest upgrade near included limit")
	}
	camera := resp.Quotas[UsageCameraActivations]
	if camera.Remaining != 1 {
		t.Fatalf("unexpected camera remaining quota: %+v", camera)
	}
	videoCalls, ok := resp.Quotas[UsageVideoCallMinutes]
	if !ok {
		t.Fatal("expected video_calls_per_day entitlement in usage quotas")
	}
	if videoCalls.Limit != 5 || videoCalls.Used != 2 ||
		videoCalls.Remaining != 3 || videoCalls.Unit != "calls" ||
		videoCalls.ResetPeriod != "day" {
		t.Fatalf("unexpected video call quota: %+v", videoCalls)
	}
	if resp.TopUsage[0].UsageType != UsageAIMinutes {
		t.Fatalf("want AI minutes as top usage, got %+v", resp.TopUsage)
	}
}
