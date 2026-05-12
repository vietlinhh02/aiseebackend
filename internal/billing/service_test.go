package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	plans                    []Plan
	entitlements             map[string][]Entitlement
	subscription             *Subscription
	careGroup                CareGroupSummary
	caredUserIDs             []string
	events                   []SubscriptionEvent
	records                  []UsageRecordInput
	aiCostEvents             []AICostEventInput
	sessions                 []UsageSessionInput
	commits                  []CommitUsageSessionInput
	grants                   []UsageGrantInput
	providerSubscriptions    []ProviderSubscriptionInput
	subscriptionEventInputs  []SubscriptionEventInput
	billingTransactionInputs []BillingTransactionInput
	usageTotals              []UsageTotal
	grantTotals              []UsageTotal
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

func (f *fakeStore) DefaultCareGroup(
	_ context.Context,
	userID string,
) (CareGroupSummary, error) {
	if f.careGroup.ID != "" {
		return f.careGroup, nil
	}
	return CareGroupSummary{
		ID:               "group-" + userID,
		Name:             "AiSee Care Group",
		OwnerUserID:      userID,
		CurrentUserRole:  "owner",
		CanManageBilling: true,
		CaredUserCount:   1,
		MemberCount:      1,
	}, nil
}

func (f *fakeStore) CurrentSubscriptionForCareGroup(
	_ context.Context,
	_ string,
) (*Subscription, error) {
	return f.subscription, nil
}

func (f *fakeStore) CaredUserIDsForCareGroup(
	_ context.Context,
	_ string,
) ([]string, error) {
	return f.caredUserIDs, nil
}

func (f *fakeStore) SubscriptionEventsForCareGroup(
	_ context.Context,
	_ string,
	_ int,
) ([]SubscriptionEvent, error) {
	return f.events, nil
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

func (f *fakeStore) PlanByCode(
	_ context.Context,
	code string,
) (Plan, error) {
	for _, plan := range f.plans {
		if plan.Code == code {
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

func (f *fakeStore) UpsertProviderSubscription(
	_ context.Context,
	input ProviderSubscriptionInput,
) (Subscription, error) {
	f.providerSubscriptions = append(f.providerSubscriptions, input)
	var plan Plan
	for _, candidate := range f.plans {
		if candidate.ID == input.PlanID {
			plan = candidate
			break
		}
	}
	if plan.ID == "" {
		return Subscription{}, errors.New("plan not found")
	}
	return Subscription{
		ID:                     "sub-" + input.ProviderSubscriptionID,
		UserID:                 input.UserID,
		CareGroupID:            input.CareGroupID,
		PlanID:                 input.PlanID,
		PlanCode:               plan.Code,
		PlanName:               plan.Name,
		Status:                 input.Status,
		CurrentPeriodStart:     input.CurrentPeriodStart,
		CurrentPeriodEnd:       input.CurrentPeriodEnd,
		CancelAtPeriodEnd:      input.CancelAtPeriodEnd,
		Provider:               input.Provider,
		ProviderSubscriptionID: input.ProviderSubscriptionID,
		Metadata:               input.Metadata,
	}, nil
}

func (f *fakeStore) InsertSubscriptionEvent(
	_ context.Context,
	input SubscriptionEventInput,
) (SubscriptionEvent, error) {
	for _, existing := range f.subscriptionEventInputs {
		if existing.Provider == input.Provider &&
			existing.ProviderEventID == input.ProviderEventID {
			return SubscriptionEvent{
				ID:              "event-" + input.ProviderEventID,
				SubscriptionID:  input.SubscriptionID,
				CareGroupID:     input.CareGroupID,
				UserID:          input.UserID,
				EventType:       input.EventType,
				Provider:        input.Provider,
				ProviderEventID: input.ProviderEventID,
				Payload:         input.Payload,
				OccurredAt:      input.OccurredAt,
				CreatedAt:       input.OccurredAt,
			}, nil
		}
	}
	f.subscriptionEventInputs = append(f.subscriptionEventInputs, input)
	return SubscriptionEvent{
		ID:              "event-" + input.ProviderEventID,
		SubscriptionID:  input.SubscriptionID,
		CareGroupID:     input.CareGroupID,
		UserID:          input.UserID,
		EventType:       input.EventType,
		Provider:        input.Provider,
		ProviderEventID: input.ProviderEventID,
		Payload:         input.Payload,
		OccurredAt:      input.OccurredAt,
		CreatedAt:       input.OccurredAt,
	}, nil
}

func (f *fakeStore) InsertBillingTransaction(
	_ context.Context,
	input BillingTransactionInput,
) (BillingTransaction, error) {
	for _, existing := range f.billingTransactionInputs {
		if existing.Provider == input.Provider &&
			existing.ProviderEventID == input.ProviderEventID {
			return BillingTransaction{
				ID:              "txn-" + input.ProviderEventID,
				SubscriptionID:  input.SubscriptionID,
				CareGroupID:     input.CareGroupID,
				UserID:          input.UserID,
				PlanID:          input.PlanID,
				Provider:        input.Provider,
				ProviderEventID: input.ProviderEventID,
				Status:          input.Status,
				Amount:          input.Amount,
				Currency:        input.Currency,
				Payload:         input.Payload,
				OccurredAt:      input.OccurredAt,
				CreatedAt:       input.OccurredAt,
			}, nil
		}
	}
	f.billingTransactionInputs = append(f.billingTransactionInputs, input)
	return BillingTransaction{
		ID:              "txn-" + input.ProviderEventID,
		SubscriptionID:  input.SubscriptionID,
		CareGroupID:     input.CareGroupID,
		UserID:          input.UserID,
		PlanID:          input.PlanID,
		Provider:        input.Provider,
		ProviderEventID: input.ProviderEventID,
		Status:          input.Status,
		Amount:          input.Amount,
		Currency:        input.Currency,
		Payload:         input.Payload,
		OccurredAt:      input.OccurredAt,
		CreatedAt:       input.OccurredAt,
	}, nil
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

func (f *fakeStore) RecordAICostEvent(
	_ context.Context,
	input AICostEventInput,
) (AICostEvent, error) {
	f.aiCostEvents = append(f.aiCostEvents, input)
	return AICostEvent{
		ID:                 "ai-cost-1",
		UserID:             input.UserID,
		UsageLedgerID:      input.UsageLedgerID,
		SessionID:          input.SessionID,
		Provider:           input.Provider,
		Model:              input.Model,
		Modality:           input.Modality,
		Source:             input.Source,
		SourceID:           input.SourceID,
		InputTokens:        input.InputTokens,
		OutputTokens:       input.OutputTokens,
		AudioInputSeconds:  input.AudioInputSeconds,
		AudioOutputSeconds: input.AudioOutputSeconds,
		ImageCount:         input.ImageCount,
		VideoSeconds:       input.VideoSeconds,
		CostUSD:            input.CostUSD,
		CostVND:            input.CostVND,
		Currency:           input.Currency,
		Estimated:          input.Estimated,
		PricingVersion:     input.PricingVersion,
		OccurredAt:         input.OccurredAt,
		Metadata:           input.Metadata,
		CreatedAt:          input.OccurredAt,
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

func (f *fakeStore) UsageTotalsForUsers(
	_ context.Context,
	_ []string,
	_, _ time.Time,
) ([]UsageTotal, error) {
	return f.usageTotals, nil
}

func (f *fakeStore) UsageGrantTotalsForUsers(
	_ context.Context,
	_ []string,
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

func TestServiceCurrentBillingOverviewUsesCareGroupSubscriptionAndUsage(
	t *testing.T,
) {
	store := &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-free",
				Code:          "free",
				Name:          "Free / Essential",
				PriceVND:      0,
				BillingPeriod: "month",
				Status:        "active",
			},
			{
				ID:            "plan-plus",
				Code:          "plus_399k",
				Name:          "Care Plus",
				PriceVND:      399000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		entitlements: map[string][]Entitlement{
			"plan-plus": {
				{
					FeatureKey:  "ai_minutes_per_month",
					LimitValue:  float64(600),
					LimitUnit:   "minutes",
					ResetPeriod: "month",
				},
				{
					FeatureKey: "weekly_report_enabled",
					LimitValue: true,
				},
			},
		},
		subscription: &Subscription{
			ID:          "sub-plus",
			CareGroupID: "group-1",
			PlanID:      "plan-plus",
			PlanCode:    "plus_399k",
			Status:      "active",
			Provider:    "google_play",
		},
		careGroup: CareGroupSummary{
			ID:               "group-1",
			Name:             "Gia dinh me",
			OwnerUserID:      "family-1",
			CurrentUserRole:  "billing_admin",
			CanManageBilling: true,
			CaredUserCount:   2,
			MemberCount:      3,
		},
		caredUserIDs: []string{"user-1", "user-2"},
		usageTotals: []UsageTotal{
			{
				UsageType: UsageAIMinutes,
				Quantity:  480,
				Unit:      "minutes",
			},
		},
		events: []SubscriptionEvent{
			{
				ID:        "event-1",
				EventType: "subscription_renewed",
				Provider:  "google_play",
				CreatedAt: time.Date(2026, 4, 26, 1, 2, 3, 0, time.UTC),
			},
		},
	}

	resp, err := NewService(store).CurrentBillingOverview(
		context.Background(),
		"family-1",
	)
	if err != nil {
		t.Fatalf("CurrentBillingOverview() error: %v", err)
	}

	if resp.CareGroup.ID != "group-1" || !resp.CareGroup.CanManageBilling {
		t.Fatalf("unexpected care group: %+v", resp.CareGroup)
	}
	if resp.EffectivePlan.Code != "plus_399k" {
		t.Fatalf("want plus plan, got %q", resp.EffectivePlan.Code)
	}
	quota := resp.Usage.Quotas[UsageAIMinutes]
	if quota.Limit != 600 || quota.Used != 480 || quota.Remaining != 120 {
		t.Fatalf("unexpected AI quota: %+v", quota)
	}
	if got := resp.Entitlements["weekly_report_enabled"].LimitValue; got != true {
		t.Fatalf("missing weekly report entitlement: %+v", resp.Entitlements)
	}
	if len(resp.Events) != 1 || resp.Events[0].EventType != "subscription_renewed" {
		t.Fatalf("unexpected subscription events: %+v", resp.Events)
	}
	if resp.StoreManagement.Provider != "google_play" {
		t.Fatalf("unexpected store management: %+v", resp.StoreManagement)
	}
}

func TestStoreManagementInfoHandlesRevenueCatStoreProviders(t *testing.T) {
	cases := map[string]string{
		"revenuecat_play_store": "https://play.google.com/store/account/subscriptions",
		"revenuecat_app_store":  "https://apps.apple.com/account/subscriptions",
	}
	for provider, wantURL := range cases {
		t.Run(provider, func(t *testing.T) {
			info := storeManagementInfo(&Subscription{Provider: provider})
			if info.Provider != provider {
				t.Fatalf("provider = %q, want %q", info.Provider, provider)
			}
			if info.ManageURL != wantURL {
				t.Fatalf("manage URL = %q, want %q", info.ManageURL, wantURL)
			}
		})
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
	store := &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-free",
				Code:          "free",
				Name:          "Free / Essential",
				PriceVND:      0,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
	}
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

func TestServiceCommitUsageSessionCapsToRemainingQuota(t *testing.T) {
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
					LimitValue:  float64(60),
					LimitUnit:   "minutes",
					ResetPeriod: "month",
				},
			},
		},
		usageTotals: []UsageTotal{
			{
				UsageType: UsageAIMinutes,
				Quantity:  59,
				Unit:      "minutes",
			},
		},
	}
	svc := NewService(store)
	start := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Minute)

	record, err := svc.CommitUsageSession(
		context.Background(),
		CommitUsageSessionInput{
			UserID:    "00000000-0000-0000-0000-000000000789",
			UsageType: UsageAIMinutes,
			Unit:      "minutes",
			Source:    "agent",
			SourceID:  "livekit-room-456",
			StartedAt: start,
			EndedAt:   end,
		},
	)
	if err != nil {
		t.Fatalf("CommitUsageSession() error: %v", err)
	}
	if record.Quantity != 1 {
		t.Fatalf("want capped quantity 1, got %.2f", record.Quantity)
	}
	if len(store.commits) != 1 || store.commits[0].Quantity != 1 {
		t.Fatalf("unexpected capped commit input: %+v", store.commits)
	}
}

func TestServiceCommitUsageSessionRejectsWhenQuotaAlreadyDepleted(t *testing.T) {
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
					LimitValue:  float64(60),
					LimitUnit:   "minutes",
					ResetPeriod: "month",
				},
			},
		},
		usageTotals: []UsageTotal{
			{
				UsageType: UsageAIMinutes,
				Quantity:  60,
				Unit:      "minutes",
			},
		},
	}
	svc := NewService(store)
	start := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)

	_, err := svc.CommitUsageSession(
		context.Background(),
		CommitUsageSessionInput{
			UserID:    "00000000-0000-0000-0000-000000000789",
			UsageType: UsageAIMinutes,
			Unit:      "minutes",
			Source:    "agent",
			SourceID:  "livekit-room-789",
			StartedAt: start,
			EndedAt:   end,
		},
	)
	if err == nil {
		t.Fatal("expected depleted AI minutes quota to fail")
	}
	quotaErr, ok := err.(*QuotaExceededError)
	if !ok {
		t.Fatalf("expected QuotaExceededError, got %T: %v", err, err)
	}
	if quotaErr.Remaining != 0 || quotaErr.Required != 1 {
		t.Fatalf("unexpected quota error: %+v", quotaErr)
	}
	if len(store.commits) != 0 {
		t.Fatalf("expected no commit writes, got %+v", store.commits)
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
				UsageType: UsageVideoCalls,
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
	videoCalls, ok := resp.Quotas[UsageVideoCalls]
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

func TestServiceRequireQuotaRejectsDepletedCareGroupQuota(t *testing.T) {
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
					FeatureKey:  "video_calls_per_day",
					LimitValue:  float64(5),
					LimitUnit:   "calls",
					ResetPeriod: "day",
				},
			},
		},
		careGroup: CareGroupSummary{
			ID:               "group-1",
			Name:             "Gia dinh me",
			OwnerUserID:      "family-1",
			CurrentUserRole:  "owner",
			CanManageBilling: true,
			CaredUserCount:   2,
			MemberCount:      2,
		},
		caredUserIDs: []string{
			"00000000-0000-0000-0000-000000000111",
			"00000000-0000-0000-0000-000000000222",
		},
		usageTotals: []UsageTotal{
			{
				UsageType: UsageVideoCalls,
				Quantity:  5,
				Unit:      "calls",
			},
		},
	}

	err := NewService(store).RequireQuota(
		context.Background(),
		QuotaCheckInput{
			UserID:    "00000000-0000-0000-0000-000000000111",
			UsageType: UsageVideoCalls,
			Quantity:  1,
		},
	)
	if err == nil {
		t.Fatal("expected depleted video call quota to fail")
	}
	quotaErr, ok := err.(*QuotaExceededError)
	if !ok {
		t.Fatalf("expected QuotaExceededError, got %T: %v", err, err)
	}
	if quotaErr.Code != "quota_exceeded" ||
		quotaErr.UsageType != UsageVideoCalls ||
		quotaErr.Remaining != 0 ||
		quotaErr.Required != 1 ||
		quotaErr.PlanCode != "basic_149k" ||
		quotaErr.ResetPeriod != "day" {
		t.Fatalf("unexpected quota error: %+v", quotaErr)
	}
}

func TestServiceRecordBillableUsageChecksQuotaBeforeLedgerWrite(t *testing.T) {
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
					FeatureKey:  "camera_activations_per_day",
					LimitValue:  float64(3),
					LimitUnit:   "activations",
					ResetPeriod: "day",
				},
			},
		},
		usageTotals: []UsageTotal{
			{
				UsageType: UsageCameraActivations,
				Quantity:  2,
				Unit:      "activations",
			},
		},
	}
	svc := NewService(store)

	record, err := svc.RecordBillableUsage(
		context.Background(),
		UsageRecordInput{
			UserID:    "00000000-0000-0000-0000-000000000789",
			UsageType: UsageCameraActivations,
			Quantity:  1,
			Unit:      "activations",
			Source:    "agent",
			SourceID:  "camera-enable-1",
		},
	)
	if err != nil {
		t.Fatalf("RecordBillableUsage() error: %v", err)
	}
	if record.UsageType != UsageCameraActivations {
		t.Fatalf("unexpected usage record: %+v", record)
	}
	if len(store.records) != 1 {
		t.Fatalf("want 1 stored record, got %d", len(store.records))
	}

	store.usageTotals = []UsageTotal{
		{
			UsageType: UsageCameraActivations,
			Quantity:  3,
			Unit:      "activations",
		},
	}
	_, err = svc.RecordBillableUsage(
		context.Background(),
		UsageRecordInput{
			UserID:    "00000000-0000-0000-0000-000000000789",
			UsageType: UsageCameraActivations,
			Quantity:  1,
			Unit:      "activations",
			Source:    "agent",
			SourceID:  "camera-enable-2",
		},
	)
	if err == nil {
		t.Fatal("expected depleted camera quota to fail")
	}
	if len(store.records) != 1 {
		t.Fatalf("depleted quota should not write ledger, got %d records", len(store.records))
	}
}

func TestServiceRecordAICostEventValidatesRequiredFields(t *testing.T) {
	svc := NewService(&fakeStore{})

	_, err := svc.RecordAICostEvent(
		context.Background(),
		AICostEventInput{
			UserID:   "",
			Provider: "google",
			Model:    "gemini-3.1-flash-live-preview",
			Source:   "agent",
			SourceID: "room-1",
			CostUSD:  0.12,
		},
	)
	if err == nil {
		t.Fatal("expected missing user_id to fail")
	}

	_, err = svc.RecordAICostEvent(
		context.Background(),
		AICostEventInput{
			UserID:   "00000000-0000-0000-0000-000000000123",
			Provider: "google",
			Model:    "",
			Source:   "agent",
			SourceID: "room-1",
			CostUSD:  -0.01,
		},
	)
	if err == nil {
		t.Fatal("expected missing model and negative cost to fail")
	}
}

func TestServiceRecordAICostEventDelegatesValidPayload(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	occurredAt := time.Date(2026, 4, 29, 9, 30, 0, 0, time.UTC)

	event, err := svc.RecordAICostEvent(
		context.Background(),
		AICostEventInput{
			UserID:             "00000000-0000-0000-0000-000000000123",
			Provider:           "google",
			Model:              "gemini-3.1-flash-live-preview",
			Modality:           "live_audio",
			Source:             "agent",
			SourceID:           "user-00000000-0000-0000-0000-000000000123",
			InputTokens:        1200,
			OutputTokens:       800,
			AudioInputSeconds:  90,
			AudioOutputSeconds: 30,
			CostUSD:            0.018,
			CostVND:            456.3,
			Currency:           "USD",
			Estimated:          true,
			PricingVersion:     "gemini-live-2026-04",
			OccurredAt:         occurredAt,
			Metadata:           map[string]any{"engine": "google_sdk"},
		},
	)
	if err != nil {
		t.Fatalf("RecordAICostEvent() error: %v", err)
	}
	if event.ID == "" {
		t.Fatalf("expected stored event id, got %+v", event)
	}
	if len(store.aiCostEvents) != 1 {
		t.Fatalf("want 1 stored cost event, got %d", len(store.aiCostEvents))
	}
	stored := store.aiCostEvents[0]
	if stored.UserID != "00000000-0000-0000-0000-000000000123" ||
		stored.Model != "gemini-3.1-flash-live-preview" ||
		stored.CostUSD != 0.018 ||
		!stored.Estimated ||
		stored.OccurredAt != occurredAt {
		t.Fatalf("unexpected stored event: %+v", stored)
	}
}

func TestServiceRecordAICostEventPricesGemini31LiveFromTokenMetadata(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)

	_, err := svc.RecordAICostEvent(
		context.Background(),
		AICostEventInput{
			UserID:   "00000000-0000-0000-0000-000000000123",
			Provider: "google",
			Model:    "gemini-3.1-flash-live-preview",
			Modality: "live_audio",
			Source:   "agent",
			SourceID: "room-1",
			Metadata: map[string]any{
				"raw_usage_metadata": []any{
					map[string]any{
						"prompt_tokens_details": []any{
							map[string]any{
								"modality":    "AUDIO",
								"token_count": 100000.0,
							},
							map[string]any{
								"modality":    "TEXT",
								"token_count": 50000.0,
							},
						},
						"candidates_tokens_details": []any{
							map[string]any{
								"modality":    "AUDIO",
								"token_count": 200000.0,
							},
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("RecordAICostEvent() error: %v", err)
	}
	stored := store.aiCostEvents[0]
	if stored.CostUSD != 2.7375 {
		t.Fatalf("cost = %v, want 2.7375", stored.CostUSD)
	}
	if stored.CostVND != 69395.63 {
		t.Fatalf("cost_vnd = %v, want 69395.63", stored.CostVND)
	}
	if stored.Estimated {
		t.Fatal("token-metadata pricing should not be marked estimated")
	}
	if stored.PricingVersion != gemini31LivePricingVersion {
		t.Fatalf("pricing version = %q", stored.PricingVersion)
	}
}

func TestServiceRecordAICostEventEstimatesGemini31LiveFromSeconds(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)

	_, err := svc.RecordAICostEvent(
		context.Background(),
		AICostEventInput{
			UserID:             "00000000-0000-0000-0000-000000000123",
			Provider:           "google",
			Model:              "gemini-3.1-flash-live-preview",
			Modality:           "live_audio",
			Source:             "agent",
			SourceID:           "room-1",
			AudioInputSeconds:  60,
			AudioOutputSeconds: 30,
		},
	)
	if err != nil {
		t.Fatalf("RecordAICostEvent() error: %v", err)
	}
	stored := store.aiCostEvents[0]
	if stored.CostUSD != 0.014 {
		t.Fatalf("cost = %v, want 0.014", stored.CostUSD)
	}
	if !stored.Estimated {
		t.Fatal("seconds fallback should be marked estimated")
	}
	if stored.PricingVersion != gemini31LivePricingVersion {
		t.Fatalf("pricing version = %q", stored.PricingVersion)
	}
}
