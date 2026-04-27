package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const revenueCatProvider = "revenuecat"

type RevenueCatConfig struct {
	ProductPlanMap string
}

type RevenueCatWebhookPayload struct {
	API    string          `json:"api_version,omitempty"`
	Event  RevenueCatEvent `json:"event"`
	RawMap map[string]any  `json:"-"`
}

type RevenueCatEvent struct {
	ID                    string                                   `json:"id"`
	Type                  string                                   `json:"type"`
	AppUserID             string                                   `json:"app_user_id"`
	ProductID             string                                   `json:"product_id"`
	EntitlementIDs        []string                                 `json:"entitlement_ids"`
	Store                 string                                   `json:"store"`
	TransactionID         string                                   `json:"transaction_id"`
	OriginalTransactionID string                                   `json:"original_transaction_id"`
	EventTimestampMS      int64                                    `json:"event_timestamp_ms"`
	PurchasedAtMS         int64                                    `json:"purchased_at_ms"`
	ExpirationAtMS        *int64                                   `json:"expiration_at_ms"`
	SubscriberAttributes  map[string]RevenueCatSubscriberAttribute `json:"subscriber_attributes"`
}

type RevenueCatSubscriberAttribute struct {
	Value       string `json:"value"`
	UpdatedAtMS int64  `json:"updated_at_ms,omitempty"`
}

type RevenueCatSyncResult struct {
	Event        SubscriptionEvent `json:"event"`
	Subscription *Subscription     `json:"subscription,omitempty"`
	PlanCode     string            `json:"plan_code,omitempty"`
	Ignored      bool              `json:"ignored"`
}

func (cfg RevenueCatConfig) PlanCodeForProduct(productID string) string {
	normalized := normalizeRevenueCatProductID(productID)
	if normalized == "" {
		return ""
	}
	for key, value := range parseProductPlanMap(cfg.ProductPlanMap) {
		if normalizeRevenueCatProductID(key) == normalized {
			return value
		}
	}
	switch normalized {
	case "aisee_basic_149k_monthly", "basic_149k_monthly", "basic_149k", "care_basic_149k", "care_basic":
		return "basic_149k"
	case "aisee_plus_399k_monthly", "plus_399k_monthly", "plus_399k", "care_plus_399k", "care_plus":
		return "plus_399k"
	case "aisee_premium_599k_monthly", "premium_599k_monthly", "premium_599k", "care_premium_599k", "care_premium":
		return "premium_599k"
	default:
		return ""
	}
}

func (s *Service) SyncRevenueCatWebhook(
	ctx context.Context,
	payload RevenueCatWebhookPayload,
) (RevenueCatSyncResult, error) {
	return s.SyncRevenueCatWebhookWithConfig(ctx, payload, RevenueCatConfig{})
}

func (s *Service) SyncRevenueCatWebhookWithConfig(
	ctx context.Context,
	payload RevenueCatWebhookPayload,
	cfg RevenueCatConfig,
) (RevenueCatSyncResult, error) {
	event := payload.Event
	if strings.TrimSpace(event.ID) == "" {
		return RevenueCatSyncResult{}, fmt.Errorf("revenuecat event id is required")
	}
	if strings.TrimSpace(event.AppUserID) == "" {
		return RevenueCatSyncResult{}, fmt.Errorf("revenuecat app_user_id is required")
	}

	provider := revenueCatProviderName(event.Store)
	occurredAt := revenueCatTime(event.EventTimestampMS, time.Now().UTC())
	result := RevenueCatSyncResult{Ignored: revenueCatEventDoesNotChangeSubscription(event.Type)}
	var sub *Subscription
	var planCode string

	if !result.Ignored {
		planCode = cfg.PlanCodeForProduct(event.ProductID)
		if planCode == "" {
			return RevenueCatSyncResult{}, fmt.Errorf(
				"unmapped revenuecat product_id: %s", event.ProductID,
			)
		}
		plan, err := s.store.PlanByCode(ctx, planCode)
		if err != nil {
			return RevenueCatSyncResult{}, fmt.Errorf("revenuecat plan lookup: %w", err)
		}
		careGroupID := revenueCatCareGroupID(event)
		if careGroupID == "" {
			group, err := s.store.DefaultCareGroup(ctx, event.AppUserID)
			if err != nil {
				return RevenueCatSyncResult{}, fmt.Errorf("revenuecat care group: %w", err)
			}
			careGroupID = group.ID
		}
		currentSub, err := s.store.CurrentSubscriptionForCareGroup(ctx, careGroupID)
		if err != nil {
			return RevenueCatSyncResult{}, fmt.Errorf("revenuecat current subscription: %w", err)
		}
		if revenueCatShouldIgnoreLowerPlanLifecycle(
			event.Type, currentSub, planCode,
		) {
			result.Ignored = true
			sub = currentSub
			result.Subscription = currentSub
			result.PlanCode = currentSub.PlanCode
		} else {
			start := revenueCatOptionalTime(event.PurchasedAtMS)
			end := revenueCatOptionalTimePtr(event.ExpirationAtMS)
			providerSubscriptionID := event.OriginalTransactionID
			if strings.TrimSpace(providerSubscriptionID) == "" {
				providerSubscriptionID = event.TransactionID
			}
			if strings.TrimSpace(providerSubscriptionID) == "" {
				providerSubscriptionID = event.ID
			}
			next, err := s.store.UpsertProviderSubscription(ctx, ProviderSubscriptionInput{
				UserID:                 event.AppUserID,
				CareGroupID:            careGroupID,
				PlanID:                 plan.ID,
				Status:                 revenueCatSubscriptionStatus(event.Type),
				CurrentPeriodStart:     start,
				CurrentPeriodEnd:       end,
				CancelAtPeriodEnd:      revenueCatCancelAtPeriodEnd(event.Type),
				Provider:               provider,
				ProviderSubscriptionID: providerSubscriptionID,
				Metadata: map[string]any{
					"revenuecat_event_id":       event.ID,
					"revenuecat_event_type":     event.Type,
					"revenuecat_product_id":     event.ProductID,
					"revenuecat_transaction_id": event.TransactionID,
					"revenuecat_entitlements":   event.EntitlementIDs,
					"revenuecat_store":          event.Store,
				},
			})
			if err != nil {
				return RevenueCatSyncResult{}, fmt.Errorf("revenuecat upsert subscription: %w", err)
			}
			sub = &next
			result.Subscription = sub
			result.PlanCode = planCode
		}
	}

	eventInput := SubscriptionEventInput{
		EventType:       "revenuecat_" + strings.ToLower(event.Type),
		Provider:        provider,
		ProviderEventID: event.ID,
		UserID:          event.AppUserID,
		OccurredAt:      occurredAt,
		Payload:         sanitizedRevenueCatPayload(payload),
	}
	if sub != nil {
		eventInput.SubscriptionID = sub.ID
		eventInput.CareGroupID = sub.CareGroupID
	}
	recordedEvent, err := s.store.InsertSubscriptionEvent(ctx, eventInput)
	if err != nil {
		return RevenueCatSyncResult{}, fmt.Errorf("revenuecat record event: %w", err)
	}
	result.Event = recordedEvent
	return result, nil
}

func parseProductPlanMap(raw string) map[string]string {
	result := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

func normalizeRevenueCatProductID(productID string) string {
	productID = strings.ToLower(strings.TrimSpace(productID))
	if productID == "" {
		return ""
	}
	productID, _, _ = strings.Cut(productID, ":")
	return strings.ReplaceAll(productID, "-", "_")
}

func revenueCatProviderName(store string) string {
	switch strings.ToUpper(strings.TrimSpace(store)) {
	case "APP_STORE", "MAC_APP_STORE":
		return "revenuecat_app_store"
	case "PLAY_STORE":
		return "revenuecat_play_store"
	default:
		return revenueCatProvider
	}
}

func revenueCatSubscriptionStatus(eventType string) string {
	switch strings.ToUpper(strings.TrimSpace(eventType)) {
	case "EXPIRATION", "SUBSCRIPTION_PAUSED":
		return "expired"
	case "BILLING_ISSUE":
		return "past_due"
	case "CANCELLATION":
		return "active"
	default:
		return "active"
	}
}

func revenueCatCancelAtPeriodEnd(eventType string) bool {
	return strings.ToUpper(strings.TrimSpace(eventType)) == "CANCELLATION"
}

func revenueCatEventDoesNotChangeSubscription(eventType string) bool {
	switch strings.ToUpper(strings.TrimSpace(eventType)) {
	case "TRANSFER", "SUBSCRIBER_ALIAS":
		return true
	default:
		return false
	}
}

func revenueCatShouldIgnoreLowerPlanLifecycle(
	eventType string,
	currentSub *Subscription,
	incomingPlanCode string,
) bool {
	if currentSub == nil {
		return false
	}
	if !revenueCatLifecycleCanArriveFromPreviousPlan(eventType) {
		return false
	}
	return planRank(currentSub.PlanCode) > planRank(incomingPlanCode)
}

func revenueCatLifecycleCanArriveFromPreviousPlan(eventType string) bool {
	switch strings.ToUpper(strings.TrimSpace(eventType)) {
	case "CANCELLATION", "EXPIRATION", "BILLING_ISSUE", "SUBSCRIPTION_PAUSED":
		return true
	default:
		return false
	}
}

func planRank(planCode string) int {
	switch strings.ToLower(strings.TrimSpace(planCode)) {
	case "premium_599k":
		return 30
	case "plus_399k":
		return 20
	case "basic_149k":
		return 10
	default:
		return 0
	}
}

func revenueCatCareGroupID(event RevenueCatEvent) string {
	for _, key := range []string{"care_group_id", "aisee_care_group_id"} {
		attr, ok := event.SubscriberAttributes[key]
		if ok && strings.TrimSpace(attr.Value) != "" {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func revenueCatOptionalTime(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

func revenueCatOptionalTimePtr(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	return revenueCatOptionalTime(*ms)
}

func revenueCatTime(ms int64, fallback time.Time) time.Time {
	t := revenueCatOptionalTime(ms)
	if t == nil {
		return fallback
	}
	return *t
}

func sanitizedRevenueCatPayload(payload RevenueCatWebhookPayload) map[string]any {
	var raw map[string]any
	b, err := json.Marshal(payload)
	if err == nil {
		_ = json.Unmarshal(b, &raw)
	}
	if raw == nil {
		raw = map[string]any{}
	}
	deleteRevenueCatSecrets(raw)
	return raw
}

func deleteRevenueCatSecrets(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") ||
				strings.Contains(lower, "secret") ||
				strings.Contains(lower, "authorization") {
				delete(typed, key)
				continue
			}
			deleteRevenueCatSecrets(child)
		}
	case []any:
		for _, child := range typed {
			deleteRevenueCatSecrets(child)
		}
	}
}
