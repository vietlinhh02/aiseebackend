package billing

import (
	"context"
	"testing"
	"time"
)

func TestRevenueCatProductPlanMapDefaultsRoadmapProducts(t *testing.T) {
	cfg := RevenueCatConfig{}

	cases := map[string]string{
		"aisee_basic_149k_monthly":   "basic_149k",
		"aisee_plus_399k_monthly":    "plus_399k",
		"aisee_premium_599k_monthly": "premium_599k",
		"basic_149k":                 "basic_149k",
		"plus_399k":                  "plus_399k",
		"premium_599k":               "premium_599k",
	}
	for productID, want := range cases {
		t.Run(productID, func(t *testing.T) {
			if got := cfg.PlanCodeForProduct(productID); got != want {
				t.Fatalf("product mapped to %q, want %q", got, want)
			}
		})
	}
}

func TestRevenueCatProductPlanMapParsesOverrides(t *testing.T) {
	cfg := RevenueCatConfig{
		ProductPlanMap: "rc_basic=basic_149k,rc_plus=plus_399k,rc_premium=premium_599k",
	}

	if got := cfg.PlanCodeForProduct("rc_basic"); got != "basic_149k" {
		t.Fatalf("rc_basic mapped to %q", got)
	}
	if got := cfg.PlanCodeForProduct("rc_plus:base"); got != "plus_399k" {
		t.Fatalf("rc_plus base plan mapped to %q", got)
	}
	if got := cfg.PlanCodeForProduct("rc_premium"); got != "premium_599k" {
		t.Fatalf("rc_premium mapped to %q", got)
	}
}

func TestSyncRevenueCatWebhookActivatesCareGroupSubscription(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
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
			{
				ID:            "plan-pro",
				Code:          "premium_599k",
				Name:          "Care Premium",
				PriceVND:      599000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		careGroup: CareGroupSummary{
			ID:               "group-1",
			Name:             "Gia dinh me",
			OwnerUserID:      "00000000-0000-0000-0000-000000000123",
			CurrentUserRole:  "billing_admin",
			CanManageBilling: true,
			CaredUserCount:   1,
			MemberCount:      2,
		},
	}
	svc := NewService(store)

	result, err := svc.SyncRevenueCatWebhook(context.Background(), RevenueCatWebhookPayload{
		Event: RevenueCatEvent{
			ID:                    "evt-1",
			Type:                  "INITIAL_PURCHASE",
			AppUserID:             "00000000-0000-0000-0000-000000000123",
			ProductID:             "aisee_plus_399k_monthly",
			TransactionID:         "txn-1",
			OriginalTransactionID: "orig-txn-1",
			Store:                 "PLAY_STORE",
			EntitlementIDs:        []string{"AiSee Pro"},
			EventTimestampMS:      now.UnixMilli(),
			PurchasedAtMS:         now.UnixMilli(),
			ExpirationAtMS:        ptrInt64(now.Add(30 * 24 * time.Hour).UnixMilli()),
			SubscriberAttributes: map[string]RevenueCatSubscriberAttribute{
				"care_group_id": {Value: "group-1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SyncRevenueCatWebhook() error: %v", err)
	}
	if result.Subscription == nil {
		t.Fatal("expected subscription")
	}
	if result.Subscription.Status != "active" {
		t.Fatalf("status = %q", result.Subscription.Status)
	}
	if result.Subscription.PlanCode != "plus_399k" {
		t.Fatalf("plan = %q", result.Subscription.PlanCode)
	}
	if result.Subscription.Provider != "revenuecat_play_store" {
		t.Fatalf("provider = %q", result.Subscription.Provider)
	}
	if len(store.providerSubscriptions) != 1 {
		t.Fatalf("want one upsert, got %d", len(store.providerSubscriptions))
	}
	input := store.providerSubscriptions[0]
	if input.CareGroupID != "group-1" {
		t.Fatalf("care group = %q", input.CareGroupID)
	}
	if input.ProviderSubscriptionID != "orig-txn-1" {
		t.Fatalf("provider subscription = %q", input.ProviderSubscriptionID)
	}
	if len(store.subscriptionEventInputs) != 1 {
		t.Fatalf("want one event, got %d", len(store.subscriptionEventInputs))
	}
	if store.subscriptionEventInputs[0].ProviderEventID != "evt-1" {
		t.Fatalf("provider event id = %q", store.subscriptionEventInputs[0].ProviderEventID)
	}
}

func TestSyncRevenueCatWebhookCancellationKeepsAccessUntilPeriodEnd(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	exp := now.Add(7 * 24 * time.Hour)
	store := revenueCatTestStore()
	svc := NewService(store)

	result, err := svc.SyncRevenueCatWebhook(context.Background(), RevenueCatWebhookPayload{
		Event: RevenueCatEvent{
			ID:                    "evt-cancel",
			Type:                  "CANCELLATION",
			AppUserID:             "00000000-0000-0000-0000-000000000123",
			ProductID:             "aisee_premium_599k_monthly",
			OriginalTransactionID: "orig-txn-2",
			Store:                 "APP_STORE",
			EventTimestampMS:      now.UnixMilli(),
			ExpirationAtMS:        ptrInt64(exp.UnixMilli()),
		},
	})
	if err != nil {
		t.Fatalf("SyncRevenueCatWebhook() error: %v", err)
	}
	if result.Subscription.Status != "active" {
		t.Fatalf("status = %q", result.Subscription.Status)
	}
	if !result.Subscription.CancelAtPeriodEnd {
		t.Fatal("expected cancel_at_period_end")
	}
}

func TestSyncRevenueCatWebhookExpirationRevokesSubscription(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	store := revenueCatTestStore()
	svc := NewService(store)

	result, err := svc.SyncRevenueCatWebhook(context.Background(), RevenueCatWebhookPayload{
		Event: RevenueCatEvent{
			ID:                    "evt-expire",
			Type:                  "EXPIRATION",
			AppUserID:             "00000000-0000-0000-0000-000000000123",
			ProductID:             "aisee_basic_149k_monthly",
			OriginalTransactionID: "orig-txn-3",
			Store:                 "PLAY_STORE",
			EventTimestampMS:      now.UnixMilli(),
			ExpirationAtMS:        ptrInt64(now.UnixMilli()),
		},
	})
	if err != nil {
		t.Fatalf("SyncRevenueCatWebhook() error: %v", err)
	}
	if result.Subscription.Status != "expired" {
		t.Fatalf("status = %q", result.Subscription.Status)
	}
}

func TestSyncRevenueCatWebhookIgnoresLowerPlanLifecycleAfterUpgrade(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	store := revenueCatTestStore()
	store.subscription = &Subscription{
		ID:                     "sub-current",
		UserID:                 "00000000-0000-0000-0000-000000000123",
		CareGroupID:            "group-1",
		PlanID:                 "plan-plus",
		PlanCode:               "plus_399k",
		PlanName:               "Care Plus",
		Status:                 "active",
		Provider:               "revenuecat_play_store",
		ProviderSubscriptionID: "orig-txn-upgrade",
	}
	svc := NewService(store)

	result, err := svc.SyncRevenueCatWebhook(context.Background(), RevenueCatWebhookPayload{
		Event: RevenueCatEvent{
			ID:                    "evt-old-basic-cancel",
			Type:                  "CANCELLATION",
			AppUserID:             "00000000-0000-0000-0000-000000000123",
			ProductID:             "aisee_basic_149k_monthly",
			OriginalTransactionID: "orig-txn-upgrade",
			Store:                 "PLAY_STORE",
			EventTimestampMS:      now.UnixMilli(),
			SubscriberAttributes: map[string]RevenueCatSubscriberAttribute{
				"care_group_id": {Value: "group-1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SyncRevenueCatWebhook() error: %v", err)
	}
	if !result.Ignored {
		t.Fatal("expected lower-plan lifecycle event to be ignored")
	}
	if len(store.providerSubscriptions) != 0 {
		t.Fatalf("expected no downgrade upsert, got %d", len(store.providerSubscriptions))
	}
	if result.Subscription == nil || result.Subscription.PlanCode != "plus_399k" {
		t.Fatalf("expected current plus subscription, got %+v", result.Subscription)
	}
	if len(store.subscriptionEventInputs) != 1 {
		t.Fatalf("want recorded event, got %d", len(store.subscriptionEventInputs))
	}
}

func TestSyncRevenueCatWebhookIgnoresTransferEventButRecordsIt(t *testing.T) {
	store := revenueCatTestStore()
	svc := NewService(store)

	result, err := svc.SyncRevenueCatWebhook(context.Background(), RevenueCatWebhookPayload{
		Event: RevenueCatEvent{
			ID:               "evt-transfer",
			Type:             "TRANSFER",
			AppUserID:        "00000000-0000-0000-0000-000000000123",
			EventTimestampMS: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC).UnixMilli(),
		},
	})
	if err != nil {
		t.Fatalf("SyncRevenueCatWebhook() error: %v", err)
	}
	if result.Subscription != nil {
		t.Fatalf("transfer should not upsert subscription: %+v", result.Subscription)
	}
	if len(store.subscriptionEventInputs) != 1 {
		t.Fatalf("want recorded event, got %d", len(store.subscriptionEventInputs))
	}
}

func TestSyncRevenueCatWebhookRecordsBillingTransactionAndDedupesEvent(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	store := revenueCatTestStore()
	svc := NewService(store)
	payload := RevenueCatWebhookPayload{
		Event: RevenueCatEvent{
			ID:                    "evt-renewal",
			Type:                  "RENEWAL",
			AppUserID:             "00000000-0000-0000-0000-000000000123",
			ProductID:             "aisee_plus_399k_monthly",
			TransactionID:         "txn-renewal-1",
			OriginalTransactionID: "orig-renewal-1",
			Store:                 "PLAY_STORE",
			EventTimestampMS:      now.UnixMilli(),
			PurchasedAtMS:         now.Add(-30 * 24 * time.Hour).UnixMilli(),
			ExpirationAtMS:        ptrInt64(now.Add(30 * 24 * time.Hour).UnixMilli()),
			SubscriberAttributes: map[string]RevenueCatSubscriberAttribute{
				"care_group_id": {Value: "group-1"},
			},
		},
	}

	result, err := svc.SyncRevenueCatWebhook(context.Background(), payload)
	if err != nil {
		t.Fatalf("SyncRevenueCatWebhook() error: %v", err)
	}
	if result.Transaction == nil {
		t.Fatal("expected billing transaction")
	}
	if result.Transaction.Status != "renewal" {
		t.Fatalf("transaction status = %q", result.Transaction.Status)
	}
	if result.Transaction.Amount != 399000 || result.Transaction.Currency != "VND" {
		t.Fatalf("unexpected transaction amount: %+v", result.Transaction)
	}
	if len(store.billingTransactionInputs) != 1 {
		t.Fatalf("want one billing transaction write, got %d", len(store.billingTransactionInputs))
	}
	tx := store.billingTransactionInputs[0]
	if tx.ProviderEventID != "evt-renewal" {
		t.Fatalf("provider event id = %q", tx.ProviderEventID)
	}
	if tx.PlanID != "plan-plus" {
		t.Fatalf("plan id = %q", tx.PlanID)
	}
	if tx.SubscriptionID == "" {
		t.Fatal("expected subscription id on billing transaction")
	}
	if len(store.subscriptionEventInputs) != 1 {
		t.Fatalf("want one subscription event write, got %d", len(store.subscriptionEventInputs))
	}

	second, err := svc.SyncRevenueCatWebhook(context.Background(), payload)
	if err != nil {
		t.Fatalf("second SyncRevenueCatWebhook() error: %v", err)
	}
	if second.Transaction == nil {
		t.Fatal("expected deduped billing transaction on replay")
	}
	if len(store.billingTransactionInputs) != 1 {
		t.Fatalf("want deduped billing transaction writes, got %d", len(store.billingTransactionInputs))
	}
	if len(store.subscriptionEventInputs) != 1 {
		t.Fatalf("want deduped subscription event writes, got %d", len(store.subscriptionEventInputs))
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

func revenueCatTestStore() *fakeStore {
	return &fakeStore{
		plans: []Plan{
			{
				ID:            "plan-basic",
				Code:          "basic_149k",
				Name:          "Care Basic",
				PriceVND:      149000,
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
			{
				ID:            "plan-pro",
				Code:          "premium_599k",
				Name:          "Care Premium",
				PriceVND:      599000,
				BillingPeriod: "month",
				Status:        "active",
			},
		},
		careGroup: CareGroupSummary{
			ID:               "group-1",
			Name:             "Gia dinh me",
			OwnerUserID:      "00000000-0000-0000-0000-000000000123",
			CurrentUserRole:  "owner",
			CanManageBilling: true,
			CaredUserCount:   1,
			MemberCount:      1,
		},
	}
}
