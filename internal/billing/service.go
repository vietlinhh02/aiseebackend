package billing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Store interface {
	ListPlans(context.Context) ([]Plan, error)
	DefaultPlan(context.Context) (Plan, error)
	DefaultCareGroup(context.Context, string) (CareGroupSummary, error)
	CurrentSubscription(context.Context, string) (*Subscription, error)
	CurrentSubscriptionForCareGroup(context.Context, string) (*Subscription, error)
	CaredUserIDsForCareGroup(context.Context, string) ([]string, error)
	SubscriptionEventsForCareGroup(
		context.Context, string, int,
	) ([]SubscriptionEvent, error)
	PlanByID(context.Context, string) (Plan, error)
	PlanByCode(context.Context, string) (Plan, error)
	EntitlementsForPlan(context.Context, string) ([]Entitlement, error)
	UpsertProviderSubscription(
		context.Context, ProviderSubscriptionInput,
	) (Subscription, error)
	InsertSubscriptionEvent(
		context.Context, SubscriptionEventInput,
	) (SubscriptionEvent, error)
	RecordUsage(context.Context, UsageRecordInput) (UsageRecord, error)
	StartUsageSession(context.Context, UsageSessionInput) (UsageSession, error)
	CommitUsageSession(context.Context, CommitUsageSessionInput) (UsageRecord, error)
	GrantUsageQuota(context.Context, UsageGrantInput) (UsageGrant, error)
	UsageTotals(context.Context, string, time.Time, time.Time) ([]UsageTotal, error)
	UsageGrantTotals(context.Context, string, time.Time, time.Time) ([]UsageTotal, error)
	UsageTotalsForUsers(context.Context, []string, time.Time, time.Time) ([]UsageTotal, error)
	UsageGrantTotalsForUsers(context.Context, []string, time.Time, time.Time) ([]UsageTotal, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListPlans(ctx context.Context) ([]Plan, error) {
	plans, err := s.store.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	for i := range plans {
		entitlements, err := s.store.EntitlementsForPlan(
			ctx, plans[i].ID,
		)
		if err != nil {
			return nil, err
		}
		plans[i].Entitlements = entitlements
	}
	return plans, nil
}

func (s *Service) CurrentSubscription(
	ctx context.Context,
	userID string,
) (SubscriptionResponse, error) {
	sub, plan, err := s.effectivePlan(ctx, userID)
	if err != nil {
		return SubscriptionResponse{}, err
	}
	return SubscriptionResponse{
		Subscription:  sub,
		EffectivePlan: summarizePlan(plan),
		AsOf:          time.Now().UTC(),
	}, nil
}

func (s *Service) CurrentEntitlements(
	ctx context.Context,
	userID string,
) (EntitlementsResponse, error) {
	_, plan, err := s.effectivePlan(ctx, userID)
	if err != nil {
		return EntitlementsResponse{}, err
	}
	entitlements, err := s.store.EntitlementsForPlan(ctx, plan.ID)
	if err != nil {
		return EntitlementsResponse{}, err
	}
	resp := EntitlementsResponse{
		Plan:         summarizePlan(plan),
		Entitlements: make(map[string]EntitlementValue, len(entitlements)),
		AsOf:         time.Now().UTC(),
	}
	for _, entitlement := range entitlements {
		resp.Entitlements[entitlement.FeatureKey] = EntitlementValue{
			LimitValue:  entitlement.LimitValue,
			LimitUnit:   entitlement.LimitUnit,
			ResetPeriod: entitlement.ResetPeriod,
			Metadata:    entitlement.Metadata,
		}
	}
	return resp, nil
}

func (s *Service) CurrentBillingOverview(
	ctx context.Context,
	userID string,
) (BillingOverviewResponse, error) {
	careGroup, err := s.store.DefaultCareGroup(ctx, userID)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	sub, plan, err := s.effectiveCareGroupPlan(ctx, careGroup.ID)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	entitlements, err := s.store.EntitlementsForPlan(ctx, plan.ID)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	entitlementMap := entitlementMap(entitlements)
	caredUserIDs, err := s.store.CaredUserIDsForCareGroup(ctx, careGroup.ID)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	if len(caredUserIDs) == 0 {
		caredUserIDs = []string{userID}
	}
	usage, err := s.usageForUsers(ctx, plan, entitlements, caredUserIDs)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	events, err := s.store.SubscriptionEventsForCareGroup(ctx, careGroup.ID, 20)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	now := time.Now().UTC()
	return BillingOverviewResponse{
		CareGroup:       careGroup,
		Subscription:    sub,
		EffectivePlan:   summarizePlan(plan),
		Entitlements:    entitlementMap,
		Usage:           usage,
		Events:          events,
		StoreManagement: storeManagementInfo(sub),
		AsOf:            now,
	}, nil
}

func (s *Service) RecordUsage(
	ctx context.Context,
	input UsageRecordInput,
) (UsageRecord, error) {
	if err := validateUsageRecord(input); err != nil {
		return UsageRecord{}, err
	}
	input.Quantity = roundQuantity(input.Quantity)
	return s.store.RecordUsage(ctx, input)
}

func (s *Service) RecordBillableUsage(
	ctx context.Context,
	input UsageRecordInput,
) (UsageRecord, error) {
	if err := validateUsageRecord(input); err != nil {
		return UsageRecord{}, err
	}
	if err := s.RequireQuota(ctx, QuotaCheckInput{
		UserID:    input.UserID,
		UsageType: input.UsageType,
		Quantity:  input.Quantity,
	}); err != nil {
		return UsageRecord{}, err
	}
	input.Quantity = roundQuantity(input.Quantity)
	return s.store.RecordUsage(ctx, input)
}

func (s *Service) RequireQuota(
	ctx context.Context,
	input QuotaCheckInput,
) error {
	decision, err := s.CheckQuota(ctx, input)
	if err != nil {
		return err
	}
	if decision.Allowed {
		return nil
	}
	return &QuotaExceededError{
		Code:        "quota_exceeded",
		UsageType:   decision.UsageType,
		Required:    decision.Required,
		Remaining:   decision.Remaining,
		PlanCode:    decision.PlanCode,
		ResetPeriod: decision.ResetPeriod,
	}
}

func (s *Service) CheckQuota(
	ctx context.Context,
	input QuotaCheckInput,
) (QuotaDecision, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return QuotaDecision{}, fmt.Errorf("user_id is required")
	}
	if !validUsageTypes[input.UsageType] {
		return QuotaDecision{}, fmt.Errorf(
			"unsupported usage_type: %s", input.UsageType,
		)
	}
	if input.Quantity <= 0 {
		return QuotaDecision{}, fmt.Errorf("quantity must be > 0")
	}
	input.Quantity = roundQuantity(input.Quantity)

	usage, err := s.currentCareGroupUsage(ctx, input.UserID)
	if err != nil {
		return QuotaDecision{}, err
	}
	quota, ok := usage.Quotas[input.UsageType]
	decision := QuotaDecision{
		Allowed:   true,
		UsageType: input.UsageType,
		Required:  input.Quantity,
		PlanCode:  usage.Plan.Code,
	}
	if !ok {
		return decision, nil
	}
	decision.Remaining = quota.Remaining
	decision.Unlimited = quota.Unlimited
	decision.ResetPeriod = quota.ResetPeriod
	decision.Quota = &quota
	if quota.Unlimited || quota.Remaining >= input.Quantity {
		return decision, nil
	}
	decision.Allowed = false
	return decision, nil
}

func (s *Service) StartUsageSession(
	ctx context.Context,
	input UsageSessionInput,
) (UsageSession, error) {
	if input.StartedAt.IsZero() {
		input.StartedAt = time.Now().UTC()
	}
	if err := validateUsageFields(
		input.UserID, input.UsageType, input.Source, input.SourceID,
	); err != nil {
		return UsageSession{}, err
	}
	if strings.TrimSpace(input.Unit) == "" {
		return UsageSession{}, fmt.Errorf("unit is required")
	}
	if input.ReservedQuantity < 0 {
		return UsageSession{}, fmt.Errorf("reserved_quantity must be >= 0")
	}
	input.ReservedQuantity = roundQuantity(input.ReservedQuantity)
	return s.store.StartUsageSession(ctx, input)
}

func (s *Service) CommitUsageSession(
	ctx context.Context,
	input CommitUsageSessionInput,
) (UsageRecord, error) {
	if err := validateUsageFields(
		input.UserID, input.UsageType, input.Source, input.SourceID,
	); err != nil {
		return UsageRecord{}, err
	}
	if strings.TrimSpace(input.Unit) == "" {
		return UsageRecord{}, fmt.Errorf("unit is required")
	}
	if input.StartedAt.IsZero() || input.EndedAt.IsZero() {
		return UsageRecord{}, fmt.Errorf("started_at and ended_at are required")
	}
	if input.EndedAt.Before(input.StartedAt) {
		return UsageRecord{}, fmt.Errorf("ended_at must be after started_at")
	}
	if input.Quantity == 0 {
		input.Quantity = durationQuantity(input.StartedAt, input.EndedAt, input.Unit)
	}
	if input.Quantity <= 0 {
		return UsageRecord{}, fmt.Errorf("quantity must be > 0")
	}
	input.Quantity = roundQuantity(input.Quantity)
	return s.store.CommitUsageSession(ctx, input)
}

func (s *Service) GrantUsageQuota(
	ctx context.Context,
	input UsageGrantInput,
) (UsageGrant, error) {
	if err := validateUsageFields(
		input.UserID, input.UsageType, input.Source, input.SourceID,
	); err != nil {
		return UsageGrant{}, err
	}
	if input.Quantity <= 0 {
		return UsageGrant{}, fmt.Errorf("quantity must be > 0")
	}
	if strings.TrimSpace(input.Unit) == "" {
		return UsageGrant{}, fmt.Errorf("unit is required")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return UsageGrant{}, fmt.Errorf("reason is required")
	}
	input.Quantity = roundQuantity(input.Quantity)
	return s.store.GrantUsageQuota(ctx, input)
}

func (s *Service) CurrentUsage(
	ctx context.Context,
	userID string,
) (UsageResponse, error) {
	_, plan, err := s.effectivePlan(ctx, userID)
	if err != nil {
		return UsageResponse{}, err
	}
	entitlements, err := s.store.EntitlementsForPlan(ctx, plan.ID)
	if err != nil {
		return UsageResponse{}, err
	}
	return s.usageForUser(ctx, userID, plan, entitlements)
}

func (s *Service) currentCareGroupUsage(
	ctx context.Context,
	userID string,
) (UsageResponse, error) {
	careGroup, err := s.store.DefaultCareGroup(ctx, userID)
	if err != nil {
		return UsageResponse{}, err
	}
	_, plan, err := s.effectiveCareGroupPlan(ctx, careGroup.ID)
	if err != nil {
		return UsageResponse{}, err
	}
	entitlements, err := s.store.EntitlementsForPlan(ctx, plan.ID)
	if err != nil {
		return UsageResponse{}, err
	}
	caredUserIDs, err := s.store.CaredUserIDsForCareGroup(ctx, careGroup.ID)
	if err != nil {
		return UsageResponse{}, err
	}
	if len(caredUserIDs) == 0 {
		caredUserIDs = []string{userID}
	}
	return s.usageForUsers(ctx, plan, entitlements, caredUserIDs)
}

func (s *Service) usageForUser(
	ctx context.Context,
	userID string,
	plan Plan,
	entitlements []Entitlement,
) (UsageResponse, error) {
	now := time.Now().UTC()
	start, end := monthBounds(now)
	usageTotals, err := s.store.UsageTotals(ctx, userID, start, end)
	if err != nil {
		return UsageResponse{}, err
	}
	grantTotals, err := s.store.UsageGrantTotals(ctx, userID, start, end)
	if err != nil {
		return UsageResponse{}, err
	}

	resp := UsageResponse{
		Plan:        summarizePlan(plan),
		Quotas:      make(map[UsageType]UsageQuota),
		TopUsage:    append([]UsageTotal(nil), usageTotals...),
		PeriodStart: start,
		PeriodEnd:   end,
		AsOf:        now,
	}
	periodCache := map[string]periodTotals{
		periodKey(start, end): {
			usageByType: totalsByType(usageTotals),
			grantByType: totalsByType(grantTotals),
		},
	}

	for _, entitlement := range entitlements {
		usageType, ok := entitlementUsageType(entitlement.FeatureKey)
		if !ok {
			continue
		}
		quotaStart, quotaEnd := boundsForResetPeriod(
			now, entitlement.ResetPeriod,
		)
		totals, err := s.totalsForPeriod(
			ctx, userID, quotaStart, quotaEnd, periodCache,
		)
		if err != nil {
			return UsageResponse{}, err
		}
		limit, unlimited := entitlementNumericLimit(entitlement.LimitValue)
		used := totals.usageByType[usageType].Quantity
		granted := totals.grantByType[usageType].Quantity
		var remaining float64
		if !unlimited {
			remaining = roundQuantity(limit + granted - used)
			if remaining < 0 {
				remaining = 0
			}
		}
		resp.Quotas[usageType] = UsageQuota{
			UsageType:        usageType,
			FeatureKey:       entitlement.FeatureKey,
			Limit:            limit,
			Used:             roundQuantity(used),
			Granted:          roundQuantity(granted),
			Remaining:        remaining,
			Unit:             entitlement.LimitUnit,
			ResetPeriod:      entitlement.ResetPeriod,
			Unlimited:        unlimited,
			UpgradeSuggested: shouldSuggestUpgrade(limit, used, unlimited),
		}
	}
	sort.Slice(resp.TopUsage, func(i, j int) bool {
		return resp.TopUsage[i].Quantity > resp.TopUsage[j].Quantity
	})
	return resp, nil
}

func (s *Service) usageForUsers(
	ctx context.Context,
	plan Plan,
	entitlements []Entitlement,
	userIDs []string,
) (UsageResponse, error) {
	now := time.Now().UTC()
	start, end := monthBounds(now)
	usageTotals, err := s.store.UsageTotalsForUsers(ctx, userIDs, start, end)
	if err != nil {
		return UsageResponse{}, err
	}
	grantTotals, err := s.store.UsageGrantTotalsForUsers(ctx, userIDs, start, end)
	if err != nil {
		return UsageResponse{}, err
	}

	resp := UsageResponse{
		Plan:        summarizePlan(plan),
		Quotas:      make(map[UsageType]UsageQuota),
		TopUsage:    append([]UsageTotal(nil), usageTotals...),
		PeriodStart: start,
		PeriodEnd:   end,
		AsOf:        now,
	}
	periodCache := map[string]periodTotals{
		periodKey(start, end): {
			usageByType: totalsByType(usageTotals),
			grantByType: totalsByType(grantTotals),
		},
	}

	for _, entitlement := range entitlements {
		usageType, ok := entitlementUsageType(entitlement.FeatureKey)
		if !ok {
			continue
		}
		quotaStart, quotaEnd := boundsForResetPeriod(
			now, entitlement.ResetPeriod,
		)
		totals, err := s.totalsForUsersPeriod(
			ctx, userIDs, quotaStart, quotaEnd, periodCache,
		)
		if err != nil {
			return UsageResponse{}, err
		}
		limit, unlimited := entitlementNumericLimit(entitlement.LimitValue)
		used := totals.usageByType[usageType].Quantity
		granted := totals.grantByType[usageType].Quantity
		var remaining float64
		if !unlimited {
			remaining = roundQuantity(limit + granted - used)
			if remaining < 0 {
				remaining = 0
			}
		}
		resp.Quotas[usageType] = UsageQuota{
			UsageType:        usageType,
			FeatureKey:       entitlement.FeatureKey,
			Limit:            limit,
			Used:             roundQuantity(used),
			Granted:          roundQuantity(granted),
			Remaining:        remaining,
			Unit:             entitlement.LimitUnit,
			ResetPeriod:      entitlement.ResetPeriod,
			Unlimited:        unlimited,
			UpgradeSuggested: shouldSuggestUpgrade(limit, used, unlimited),
		}
	}
	sort.Slice(resp.TopUsage, func(i, j int) bool {
		return resp.TopUsage[i].Quantity > resp.TopUsage[j].Quantity
	})
	return resp, nil
}

func (s *Service) totalsForPeriod(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
	cache map[string]periodTotals,
) (periodTotals, error) {
	key := periodKey(start, end)
	if totals, ok := cache[key]; ok {
		return totals, nil
	}
	usageTotals, err := s.store.UsageTotals(ctx, userID, start, end)
	if err != nil {
		return periodTotals{}, err
	}
	grantTotals, err := s.store.UsageGrantTotals(ctx, userID, start, end)
	if err != nil {
		return periodTotals{}, err
	}
	totals := periodTotals{
		usageByType: totalsByType(usageTotals),
		grantByType: totalsByType(grantTotals),
	}
	cache[key] = totals
	return totals, nil
}

func (s *Service) totalsForUsersPeriod(
	ctx context.Context,
	userIDs []string,
	start time.Time,
	end time.Time,
	cache map[string]periodTotals,
) (periodTotals, error) {
	key := periodKey(start, end)
	if totals, ok := cache[key]; ok {
		return totals, nil
	}
	usageTotals, err := s.store.UsageTotalsForUsers(ctx, userIDs, start, end)
	if err != nil {
		return periodTotals{}, err
	}
	grantTotals, err := s.store.UsageGrantTotalsForUsers(ctx, userIDs, start, end)
	if err != nil {
		return periodTotals{}, err
	}
	totals := periodTotals{
		usageByType: totalsByType(usageTotals),
		grantByType: totalsByType(grantTotals),
	}
	cache[key] = totals
	return totals, nil
}

func (s *Service) effectivePlan(
	ctx context.Context,
	userID string,
) (*Subscription, Plan, error) {
	sub, err := s.store.CurrentSubscription(ctx, userID)
	if err != nil {
		return nil, Plan{}, err
	}
	if sub != nil {
		plan, err := s.store.PlanByID(ctx, sub.PlanID)
		if err != nil {
			return nil, Plan{}, err
		}
		return sub, plan, nil
	}
	plan, err := s.store.DefaultPlan(ctx)
	if err != nil {
		return nil, Plan{}, err
	}
	return nil, plan, nil
}

func (s *Service) effectiveCareGroupPlan(
	ctx context.Context,
	careGroupID string,
) (*Subscription, Plan, error) {
	sub, err := s.store.CurrentSubscriptionForCareGroup(ctx, careGroupID)
	if err != nil {
		return nil, Plan{}, err
	}
	if sub != nil {
		plan, err := s.store.PlanByID(ctx, sub.PlanID)
		if err != nil {
			return nil, Plan{}, err
		}
		return sub, plan, nil
	}
	plan, err := s.store.DefaultPlan(ctx)
	if err != nil {
		return nil, Plan{}, err
	}
	return nil, plan, nil
}

func entitlementMap(entitlements []Entitlement) map[string]EntitlementValue {
	values := make(map[string]EntitlementValue, len(entitlements))
	for _, entitlement := range entitlements {
		values[entitlement.FeatureKey] = EntitlementValue{
			LimitValue:  entitlement.LimitValue,
			LimitUnit:   entitlement.LimitUnit,
			ResetPeriod: entitlement.ResetPeriod,
			Metadata:    entitlement.Metadata,
		}
	}
	return values
}

func storeManagementInfo(sub *Subscription) StoreManagementInfo {
	if sub == nil {
		return StoreManagementInfo{
			Instructions: "Chưa có gói trả phí. Hãy nâng gói từ ứng dụng của người thân.",
		}
	}
	switch sub.Provider {
	case "google_play", "revenuecat_play_store":
		return StoreManagementInfo{
			Provider:     sub.Provider,
			ManageURL:    "https://play.google.com/store/account/subscriptions",
			Instructions: "Quản lý hoặc hủy gói trong Google Play.",
		}
	case "app_store", "revenuecat_app_store":
		return StoreManagementInfo{
			Provider:     sub.Provider,
			ManageURL:    "https://apps.apple.com/account/subscriptions",
			Instructions: "Quản lý hoặc hủy gói trong App Store.",
		}
	default:
		return StoreManagementInfo{
			Provider:     sub.Provider,
			Instructions: "Gói này được quản lý bởi cửa hàng đã thanh toán.",
		}
	}
}

func validateUsageRecord(input UsageRecordInput) error {
	if err := validateUsageFields(
		input.UserID, input.UsageType, input.Source, input.SourceID,
	); err != nil {
		return err
	}
	if input.Quantity <= 0 {
		return fmt.Errorf("quantity must be > 0")
	}
	if strings.TrimSpace(input.Unit) == "" {
		return fmt.Errorf("unit is required")
	}
	return nil
}

func validateUsageFields(
	userID string,
	usageType UsageType,
	source string,
	sourceID string,
) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if !validUsageTypes[usageType] {
		return fmt.Errorf("unsupported usage_type: %s", usageType)
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return fmt.Errorf("source is required")
	}
	switch strings.ToLower(source) {
	case "client", "app":
		return fmt.Errorf("usage source must be backend or agent owned")
	}
	if strings.TrimSpace(sourceID) == "" {
		return fmt.Errorf("source_id is required")
	}
	return nil
}

func durationQuantity(start, end time.Time, unit string) float64 {
	duration := end.Sub(start)
	switch unit {
	case "minutes":
		return ceilQuantity(duration.Minutes())
	case "seconds":
		return ceilQuantity(duration.Seconds())
	default:
		return ceilQuantity(duration.Minutes())
	}
}

func roundQuantity(value float64) float64 {
	return math.Round(value*100) / 100
}

func ceilQuantity(value float64) float64 {
	return math.Ceil(value*100) / 100
}

func monthBounds(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func boundsForResetPeriod(now time.Time, resetPeriod string) (time.Time, time.Time) {
	switch resetPeriod {
	case "day":
		start := time.Date(
			now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC,
		)
		return start, start.AddDate(0, 0, 1)
	case "year":
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(1, 0, 0)
	case "never":
		return time.Time{}, time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	default:
		return monthBounds(now)
	}
}

func periodKey(start, end time.Time) string {
	return start.Format(time.RFC3339Nano) + "/" + end.Format(time.RFC3339Nano)
}

func totalsByType(totals []UsageTotal) map[UsageType]UsageTotal {
	byType := make(map[UsageType]UsageTotal, len(totals))
	for _, total := range totals {
		total.Quantity = roundQuantity(total.Quantity)
		byType[total.UsageType] = total
	}
	return byType
}

func entitlementUsageType(featureKey string) (UsageType, bool) {
	usageType, ok := entitlementFeatureUsageTypes[featureKey]
	return usageType, ok
}

func entitlementNumericLimit(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), false
	case int64:
		return float64(v), false
	case float64:
		return v, false
	case string:
		if v == "fair_use" || v == "unlimited" {
			return 0, true
		}
	}
	return 0, false
}

func shouldSuggestUpgrade(limit, used float64, unlimited bool) bool {
	if unlimited || limit <= 0 {
		return false
	}
	return used >= limit*0.8
}

var validUsageTypes = map[UsageType]bool{
	UsageAIMinutes:            true,
	UsageCameraActivations:    true,
	UsageVoiceCallMinutes:     true,
	UsageVideoCalls:           true,
	UsageVideoCallMinutes:     true,
	UsageMusicSessions:        true,
	UsageMusicMinutes:         true,
	UsageNavigationSessions:   true,
	UsageVolunteerMatches:     true,
	UsagePDFExports:           true,
	UsageCareCoachQuestions:   true,
	UsagePatternDetectionRuns: true,
}

type periodTotals struct {
	usageByType map[UsageType]UsageTotal
	grantByType map[UsageType]UsageTotal
}

var entitlementFeatureUsageTypes = map[string]UsageType{
	"ai_minutes_per_month":             UsageAIMinutes,
	"camera_activations_per_day":       UsageCameraActivations,
	"voice_call_minutes_per_month":     UsageVoiceCallMinutes,
	"video_calls_per_day":              UsageVideoCalls,
	"video_call_minutes_per_month":     UsageVideoCallMinutes,
	"music_sessions_per_month":         UsageMusicSessions,
	"music_minutes_per_month":          UsageMusicMinutes,
	"navigation_sessions_per_month":    UsageNavigationSessions,
	"volunteer_matches_per_month":      UsageVolunteerMatches,
	"pdf_exports_per_month":            UsagePDFExports,
	"care_coach_questions_per_month":   UsageCareCoachQuestions,
	"pattern_detection_runs_per_month": UsagePatternDetectionRuns,
}
