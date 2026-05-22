package billing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/team-dang/aiseebackend/pkg/i18n"
)

type Store interface {
	ListPlans(context.Context) ([]Plan, error)
	DefaultPlan(context.Context) (Plan, error)
	ActiveCareGroup(context.Context, string) (CareGroupSummary, error)
	DefaultCareGroup(context.Context, string) (CareGroupSummary, error)
	CareGroupForBilling(
		context.Context, BillingOverviewInput,
	) (CareGroupSummary, error)
	SubscriptionByProviderRef(
		context.Context, string, string,
	) (*Subscription, error)
	CurrentSubscription(context.Context, string) (*Subscription, error)
	CurrentSubscriptionForCareGroup(context.Context, string) (*Subscription, error)
	ActiveSubscriptionsForCareGroup(context.Context, string) ([]Subscription, error)
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
	InsertBillingTransaction(
		context.Context, BillingTransactionInput,
	) (BillingTransaction, error)
	RecordUsage(context.Context, UsageRecordInput) (UsageRecord, error)
	RecordAICostEvent(context.Context, AICostEventInput) (AICostEvent, error)
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

const defaultFallbackPlanCode = "plus_399k"
const defaultFallbackPlanName = "Trial 1 tháng"
const defaultTrialDurationMonths = 1

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
	return s.CurrentSubscriptionForBilling(ctx, BillingOverviewInput{
		UserID: userID,
	})
}

func (s *Service) CurrentSubscriptionForBilling(
	ctx context.Context,
	input BillingOverviewInput,
) (SubscriptionResponse, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return SubscriptionResponse{}, fmt.Errorf("user_id is required")
	}
	careGroup, err := s.store.CareGroupForBilling(ctx, input)
	if err != nil {
		return SubscriptionResponse{}, err
	}
	sub, plan, err := s.effectiveCareGroupPlan(ctx, careGroup)
	if err != nil {
		return SubscriptionResponse{}, err
	}
	return SubscriptionResponse{
		Subscription:  sub,
		EffectivePlan: summarizeEffectivePlan(sub, plan),
		AsOf:          time.Now().UTC(),
	}, nil
}

func (s *Service) CurrentEntitlements(
	ctx context.Context,
	userID string,
) (EntitlementsResponse, error) {
	return s.CurrentEntitlementsForBilling(ctx, BillingOverviewInput{
		UserID: userID,
	})
}

func (s *Service) CurrentEntitlementsForBilling(
	ctx context.Context,
	input BillingOverviewInput,
) (EntitlementsResponse, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return EntitlementsResponse{}, fmt.Errorf("user_id is required")
	}
	careGroup, err := s.store.CareGroupForBilling(ctx, input)
	if err != nil {
		return EntitlementsResponse{}, err
	}
	sub, plan, entitlements, err := s.effectiveCareGroupEntitlements(
		ctx, careGroup,
	)
	if err != nil {
		return EntitlementsResponse{}, err
	}
	resp := EntitlementsResponse{
		Plan:         summarizeEffectivePlan(sub, plan),
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
	input BillingOverviewInput,
) (BillingOverviewResponse, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return BillingOverviewResponse{}, fmt.Errorf("user_id is required")
	}
	careGroup, err := s.store.CareGroupForBilling(ctx, input)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	sub, plan, entitlements, err := s.effectiveCareGroupEntitlements(
		ctx, careGroup,
	)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	entitlementMap := entitlementMap(entitlements)
	caredUserIDs, err := s.store.CaredUserIDsForCareGroup(ctx, careGroup.ID)
	if err != nil {
		return BillingOverviewResponse{}, err
	}
	if len(caredUserIDs) == 0 {
		caredUserIDs = []string{input.UserID}
	}
	usage, err := s.usageForUsers(ctx, sub, plan, entitlements, caredUserIDs)
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
		EffectivePlan:   summarizeEffectivePlan(sub, plan),
		Entitlements:    entitlementMap,
		Usage:           usage,
		Events:          events,
		StoreManagement: storeManagementInfoLang(sub, i18n.FromContext(ctx)),
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
		UserID:      input.UserID,
		CareGroupID: input.CareGroupID,
		CaredUserID: input.CaredUserID,
		UsageType:   input.UsageType,
		Quantity:    input.Quantity,
	}); err != nil {
		return UsageRecord{}, err
	}
	input.Quantity = roundQuantity(input.Quantity)
	return s.store.RecordUsage(ctx, input)
}

func (s *Service) RecordAICostEvent(
	ctx context.Context,
	input AICostEventInput,
) (AICostEvent, error) {
	if err := validateAICostEvent(input); err != nil {
		return AICostEvent{}, err
	}
	if strings.TrimSpace(input.Modality) == "" {
		input.Modality = "live_audio"
	}
	if strings.TrimSpace(input.Currency) == "" {
		input.Currency = "USD"
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	input.AudioInputSeconds = roundQuantity(input.AudioInputSeconds)
	input.AudioOutputSeconds = roundQuantity(input.AudioOutputSeconds)
	input.VideoSeconds = roundQuantity(input.VideoSeconds)
	input = priceAICostEvent(input)
	return s.store.RecordAICostEvent(ctx, input)
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

	usage, err := s.currentCareGroupUsage(ctx, BillingOverviewInput{
		UserID:      input.UserID,
		CareGroupID: input.CareGroupID,
		CaredUserID: input.CaredUserID,
	})
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
		if usageTypeRequiresConfiguredPlanQuota(input.UsageType) {
			decision.Allowed = false
			decision.Remaining = 0
			return decision, nil
		}
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
	if input.ReservedQuantity > 0 {
		if err := s.RequireQuota(ctx, QuotaCheckInput{
			UserID:      input.UserID,
			CareGroupID: input.CareGroupID,
			CaredUserID: input.CaredUserID,
			UsageType:   input.UsageType,
			Quantity:    input.ReservedQuantity,
		}); err != nil {
			return UsageSession{}, err
		}
	}
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
	input, err := s.enforceUsageSessionCommitQuota(ctx, input)
	if err != nil {
		return UsageRecord{}, err
	}
	return s.store.CommitUsageSession(ctx, input)
}

func (s *Service) enforceUsageSessionCommitQuota(
	ctx context.Context,
	input CommitUsageSessionInput,
) (CommitUsageSessionInput, error) {
	decision, err := s.CheckQuota(ctx, QuotaCheckInput{
		UserID:      input.UserID,
		CareGroupID: input.CareGroupID,
		CaredUserID: input.CaredUserID,
		UsageType:   input.UsageType,
		Quantity:    input.Quantity,
	})
	if err != nil {
		return CommitUsageSessionInput{}, err
	}
	if decision.Allowed {
		return input, nil
	}
	if decision.Remaining <= 0 {
		return CommitUsageSessionInput{}, &QuotaExceededError{
			Code:        "quota_exceeded",
			UsageType:   decision.UsageType,
			Required:    input.Quantity,
			Remaining:   decision.Remaining,
			PlanCode:    decision.PlanCode,
			ResetPeriod: decision.ResetPeriod,
		}
	}

	requestedQuantity := input.Quantity
	input.Quantity = roundQuantity(decision.Remaining)
	if input.Quantity <= 0 {
		return CommitUsageSessionInput{}, &QuotaExceededError{
			Code:        "quota_exceeded",
			UsageType:   decision.UsageType,
			Required:    requestedQuantity,
			Remaining:   decision.Remaining,
			PlanCode:    decision.PlanCode,
			ResetPeriod: decision.ResetPeriod,
		}
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.Metadata["quota_capped"] = true
	input.Metadata["requested_quantity"] = requestedQuantity
	input.Metadata["committed_quantity"] = input.Quantity
	return input, nil
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
	input BillingOverviewInput,
) (UsageResponse, error) {
	if strings.TrimSpace(input.UserID) == "" {
		return UsageResponse{}, fmt.Errorf("user_id is required")
	}
	return s.currentCareGroupUsage(ctx, input)
}

func (s *Service) currentCareGroupUsage(
	ctx context.Context,
	input BillingOverviewInput,
) (UsageResponse, error) {
	careGroup, err := s.store.CareGroupForBilling(ctx, input)
	if err != nil {
		return UsageResponse{}, err
	}
	sub, plan, entitlements, err := s.effectiveCareGroupEntitlements(
		ctx, careGroup,
	)
	if err != nil {
		return UsageResponse{}, err
	}
	caredUserIDs, err := s.store.CaredUserIDsForCareGroup(ctx, careGroup.ID)
	if err != nil {
		return UsageResponse{}, err
	}
	if len(caredUserIDs) == 0 {
		caredUserIDs = []string{input.UserID}
	}
	return s.usageForUsers(ctx, sub, plan, entitlements, caredUserIDs)
}

func (s *Service) usageForUser(
	ctx context.Context,
	sub *Subscription,
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
		Plan:        summarizeEffectivePlan(sub, plan),
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
	sub *Subscription,
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
		Plan:        summarizeEffectivePlan(sub, plan),
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

func (s *Service) effectiveCareGroupPlan(
	ctx context.Context,
	careGroup CareGroupSummary,
) (*Subscription, Plan, error) {
	sub, err := s.store.CurrentSubscriptionForCareGroup(ctx, careGroup.ID)
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
	if trialSub, trialPlan, ok, err := s.activeFallbackTrial(
		ctx,
		careGroup,
		time.Now().UTC(),
	); err != nil {
		return nil, Plan{}, err
	} else if ok {
		return trialSub, trialPlan, nil
	}
	plan, err := s.store.DefaultPlan(ctx)
	if err != nil {
		return nil, Plan{}, err
	}
	return nil, plan, nil
}

func summarizeEffectivePlan(sub *Subscription, plan Plan) PlanSummary {
	summary := summarizePlan(plan)
	if isSyntheticTrialSubscription(sub) && plan.Code == defaultFallbackPlanCode {
		summary.Name = defaultFallbackPlanName
	}
	return summary
}

func (s *Service) activeFallbackTrial(
	ctx context.Context,
	careGroup CareGroupSummary,
	now time.Time,
) (*Subscription, Plan, bool, error) {
	if careGroup.CreatedAt.IsZero() {
		return nil, Plan{}, false, nil
	}
	plan, err := s.store.PlanByCode(ctx, defaultFallbackPlanCode)
	if err != nil {
		return nil, Plan{}, false, fmt.Errorf("billing trial plan lookup: %w", err)
	}
	start := careGroup.CreatedAt.UTC()
	end := start.AddDate(0, defaultTrialDurationMonths, 0)
	if !now.Before(end) {
		return nil, Plan{}, false, nil
	}
	return &Subscription{
		ID:                 "",
		CareGroupID:        careGroup.ID,
		PlanID:             plan.ID,
		PlanCode:           plan.Code,
		PlanName:           defaultFallbackPlanName,
		Status:             "trialing",
		CurrentPeriodStart: &start,
		CurrentPeriodEnd:   &end,
		Metadata: map[string]any{
			"synthetic_trial": true,
		},
	}, plan, true, nil
}

func isSyntheticTrialSubscription(sub *Subscription) bool {
	if sub == nil || sub.Status != "trialing" || sub.Metadata == nil {
		return false
	}
	flag, _ := sub.Metadata["synthetic_trial"].(bool)
	return flag
}

func (s *Service) effectiveCareGroupEntitlements(
	ctx context.Context,
	careGroup CareGroupSummary,
) (*Subscription, Plan, []Entitlement, error) {
	sub, plan, err := s.effectiveCareGroupPlan(ctx, careGroup)
	if err != nil {
		return nil, Plan{}, nil, err
	}
	subs, err := s.store.ActiveSubscriptionsForCareGroup(ctx, careGroup.ID)
	if err != nil {
		return nil, Plan{}, nil, err
	}
	if len(subs) == 0 {
		entitlements, err := s.store.EntitlementsForPlan(ctx, plan.ID)
		if err != nil {
			return nil, Plan{}, nil, err
		}
		return sub, plan, entitlements, nil
	}
	entitlements, err := s.aggregateEntitlementsForSubscriptions(ctx, subs)
	if err != nil {
		return nil, Plan{}, nil, err
	}
	return sub, plan, entitlements, nil
}

func (s *Service) aggregateEntitlementsForSubscriptions(
	ctx context.Context,
	subs []Subscription,
) ([]Entitlement, error) {
	merged := make(map[string]Entitlement)
	for _, sub := range subs {
		entitlements, err := s.store.EntitlementsForPlan(ctx, sub.PlanID)
		if err != nil {
			return nil, err
		}
		for _, entitlement := range entitlements {
			current, ok := merged[entitlement.FeatureKey]
			if !ok {
				merged[entitlement.FeatureKey] = cloneEntitlement(entitlement)
				continue
			}
			merged[entitlement.FeatureKey] = mergeEntitlementValues(
				current, entitlement,
			)
		}
	}
	result := make([]Entitlement, 0, len(merged))
	for _, entitlement := range merged {
		result = append(result, entitlement)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].FeatureKey < result[j].FeatureKey
	})
	return result, nil
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

func cloneEntitlement(entitlement Entitlement) Entitlement {
	cloned := entitlement
	if entitlement.Metadata != nil {
		cloned.Metadata = make(map[string]any, len(entitlement.Metadata))
		for key, value := range entitlement.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}

func mergeEntitlementValues(current, incoming Entitlement) Entitlement {
	merged := cloneEntitlement(current)
	if merged.LimitUnit == "" {
		merged.LimitUnit = incoming.LimitUnit
	}
	if merged.ResetPeriod == "" {
		merged.ResetPeriod = incoming.ResetPeriod
	}
	if merged.Metadata == nil && incoming.Metadata != nil {
		merged.Metadata = cloneEntitlement(incoming).Metadata
	}

	switch incomingValue := incoming.LimitValue.(type) {
	case bool:
		currentValue, _ := merged.LimitValue.(bool)
		merged.LimitValue = currentValue || incomingValue
	case int, int64, float64:
		currentLimit, currentUnlimited := entitlementNumericLimit(merged.LimitValue)
		incomingLimit, incomingUnlimited := entitlementNumericLimit(incoming.LimitValue)
		switch {
		case currentUnlimited:
			return merged
		case incomingUnlimited:
			merged.LimitValue = incoming.LimitValue
		default:
			merged.LimitValue = roundQuantity(currentLimit + incomingLimit)
		}
	case string:
		if incomingValue == "fair_use" || incomingValue == "unlimited" {
			merged.LimitValue = incoming.LimitValue
			return merged
		}
		currentString, _ := merged.LimitValue.(string)
		if compareEntitlementLevel(
			incoming.FeatureKey, incomingValue, currentString,
		) > 0 {
			merged.LimitValue = incoming.LimitValue
		}
	default:
		if merged.LimitValue == nil {
			merged.LimitValue = incoming.LimitValue
		}
	}
	return merged
}

func compareEntitlementLevel(
	featureKey string,
	candidate string,
	current string,
) int {
	if current == "" {
		return 1
	}
	ordering := entitlementLevelOrder[featureKey]
	if len(ordering) == 0 {
		if candidate == current {
			return 0
		}
		return -1
	}
	candidateRank, candidateOK := ordering[candidate]
	currentRank, currentOK := ordering[current]
	switch {
	case candidateOK && currentOK:
		return candidateRank - currentRank
	case candidateOK:
		return 1
	case currentOK:
		return -1
	default:
		return -1
	}
}

func storeManagementInfo(sub *Subscription) StoreManagementInfo {
	return storeManagementInfoLang(sub, i18n.DefaultLang)
}

func storeManagementInfoLang(sub *Subscription, lang string) StoreManagementInfo {
	if sub == nil {
		return StoreManagementInfo{
			Instructions: i18n.T(lang, "billing.instructions.no_paid_plan"),
		}
	}
	switch sub.Provider {
	case "google_play", "revenuecat_play_store":
		return StoreManagementInfo{
			Provider:     sub.Provider,
			ManageURL:    "https://play.google.com/store/account/subscriptions",
			Instructions: i18n.T(lang, "billing.instructions.google_play"),
		}
	case "app_store", "revenuecat_app_store":
		return StoreManagementInfo{
			Provider:     sub.Provider,
			ManageURL:    "https://apps.apple.com/account/subscriptions",
			Instructions: i18n.T(lang, "billing.instructions.app_store"),
		}
	default:
		return StoreManagementInfo{
			Provider:     sub.Provider,
			Instructions: i18n.T(lang, "billing.instructions.store_managed"),
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

func validateAICostEvent(input AICostEventInput) error {
	if strings.TrimSpace(input.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(input.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(input.Source) == "" {
		return fmt.Errorf("source is required")
	}
	if strings.TrimSpace(input.SourceID) == "" {
		return fmt.Errorf("source_id is required")
	}
	if input.InputTokens < 0 || input.OutputTokens < 0 ||
		input.ImageCount < 0 {
		return fmt.Errorf("token and image counters must be >= 0")
	}
	if input.AudioInputSeconds < 0 || input.AudioOutputSeconds < 0 ||
		input.VideoSeconds < 0 {
		return fmt.Errorf("media counters must be >= 0")
	}
	if input.CostUSD < 0 || input.CostVND < 0 {
		return fmt.Errorf("cost must be >= 0")
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

func roundCost(value float64) float64 {
	return math.Round(value*1000000) / 1000000
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

var entitlementLevelOrder = map[string]map[string]int{
	"permission_matrix_level": {
		"none":     0,
		"basic":    1,
		"advanced": 2,
	},
	"volunteer_priority_level": {
		"none":     0,
		"standard": 1,
		"priority": 2,
		"highest":  3,
	},
	"export_format": {
		"none":    0,
		"csv":     1,
		"pdf_csv": 2,
	},
}

func usageTypeRequiresConfiguredPlanQuota(ut UsageType) bool {
	switch ut {
	case UsageCareCoachQuestions, UsagePatternDetectionRuns, UsagePDFExports:
		return true
	default:
		return false
	}
}
