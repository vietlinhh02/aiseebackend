package billing

import "time"

type UsageType string

const (
	UsageAIMinutes            UsageType = "ai_minutes"
	UsageCameraActivations    UsageType = "camera_activations"
	UsageVoiceCallMinutes     UsageType = "voice_call_minutes"
	UsageVideoCalls           UsageType = "video_calls"
	UsageVideoCallMinutes     UsageType = "video_call_minutes"
	UsageMusicSessions        UsageType = "music_sessions"
	UsageMusicMinutes         UsageType = "music_minutes"
	UsageNavigationSessions   UsageType = "navigation_sessions"
	UsageVolunteerMatches     UsageType = "volunteer_matches"
	UsagePDFExports           UsageType = "pdf_exports"
	UsageCareCoachQuestions   UsageType = "care_coach_questions"
	UsagePatternDetectionRuns UsageType = "pattern_detection_runs"
)

type UsageRecordInput struct {
	UserID      string         `json:"user_id"`
	CareGroupID string         `json:"care_group_id,omitempty"`
	CaredUserID string         `json:"cared_user_id,omitempty"`
	UsageType   UsageType      `json:"usage_type"`
	Quantity    float64        `json:"quantity"`
	Unit        string         `json:"unit"`
	Source      string         `json:"source"`
	SourceID    string         `json:"source_id"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UsageRecord struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id,omitempty"`
	UsageType UsageType      `json:"usage_type"`
	Quantity  float64        `json:"quantity"`
	Unit      string         `json:"unit"`
	Source    string         `json:"source"`
	SourceID  string         `json:"source_id"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type AICostEventInput struct {
	UserID             string         `json:"user_id"`
	CareGroupID        string         `json:"care_group_id,omitempty"`
	CaredUserID        string         `json:"cared_user_id,omitempty"`
	UsageLedgerID      string         `json:"usage_ledger_id,omitempty"`
	SessionID          string         `json:"session_id,omitempty"`
	Provider           string         `json:"provider"`
	Model              string         `json:"model"`
	Modality           string         `json:"modality"`
	Source             string         `json:"source"`
	SourceID           string         `json:"source_id"`
	InputTokens        int64          `json:"input_tokens,omitempty"`
	OutputTokens       int64          `json:"output_tokens,omitempty"`
	AudioInputSeconds  float64        `json:"audio_input_seconds,omitempty"`
	AudioOutputSeconds float64        `json:"audio_output_seconds,omitempty"`
	ImageCount         int64          `json:"image_count,omitempty"`
	VideoSeconds       float64        `json:"video_seconds,omitempty"`
	CostUSD            float64        `json:"cost_usd"`
	CostVND            float64        `json:"cost_vnd,omitempty"`
	Currency           string         `json:"currency,omitempty"`
	Estimated          bool           `json:"estimated"`
	PricingVersion     string         `json:"pricing_version,omitempty"`
	OccurredAt         time.Time      `json:"occurred_at,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type AICostEvent struct {
	ID                 string         `json:"id"`
	UserID             string         `json:"user_id"`
	UsageLedgerID      string         `json:"usage_ledger_id,omitempty"`
	SessionID          string         `json:"session_id,omitempty"`
	Provider           string         `json:"provider"`
	Model              string         `json:"model"`
	Modality           string         `json:"modality"`
	Source             string         `json:"source"`
	SourceID           string         `json:"source_id"`
	InputTokens        int64          `json:"input_tokens,omitempty"`
	OutputTokens       int64          `json:"output_tokens,omitempty"`
	AudioInputSeconds  float64        `json:"audio_input_seconds,omitempty"`
	AudioOutputSeconds float64        `json:"audio_output_seconds,omitempty"`
	ImageCount         int64          `json:"image_count,omitempty"`
	VideoSeconds       float64        `json:"video_seconds,omitempty"`
	CostUSD            float64        `json:"cost_usd"`
	CostVND            float64        `json:"cost_vnd,omitempty"`
	Currency           string         `json:"currency,omitempty"`
	Estimated          bool           `json:"estimated"`
	PricingVersion     string         `json:"pricing_version,omitempty"`
	OccurredAt         time.Time      `json:"occurred_at"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

type QuotaCheckInput struct {
	UserID      string    `json:"user_id"`
	CareGroupID string    `json:"care_group_id,omitempty"`
	CaredUserID string    `json:"cared_user_id,omitempty"`
	UsageType   UsageType `json:"usage_type"`
	Quantity    float64   `json:"quantity"`
}

type QuotaDecision struct {
	Allowed     bool        `json:"allowed"`
	UsageType   UsageType   `json:"usage_type"`
	Required    float64     `json:"required"`
	Remaining   float64     `json:"remaining"`
	Unlimited   bool        `json:"unlimited"`
	PlanCode    string      `json:"plan_code,omitempty"`
	ResetPeriod string      `json:"reset_period,omitempty"`
	Quota       *UsageQuota `json:"quota,omitempty"`
}

type QuotaExceededError struct {
	Code        string    `json:"code"`
	UsageType   UsageType `json:"usage_type"`
	Required    float64   `json:"required"`
	Remaining   float64   `json:"remaining"`
	PlanCode    string    `json:"plan_code,omitempty"`
	ResetPeriod string    `json:"reset_period,omitempty"`
}

func (e *QuotaExceededError) Error() string {
	return "quota_exceeded"
}

type UsageSessionInput struct {
	UserID           string         `json:"user_id"`
	CareGroupID      string         `json:"care_group_id,omitempty"`
	CaredUserID      string         `json:"cared_user_id,omitempty"`
	UsageType        UsageType      `json:"usage_type"`
	ReservedQuantity float64        `json:"reserved_quantity,omitempty"`
	Unit             string         `json:"unit"`
	Source           string         `json:"source"`
	SourceID         string         `json:"source_id"`
	StartedAt        time.Time      `json:"started_at"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type UsageSession struct {
	ID                string         `json:"id"`
	UserID            string         `json:"user_id,omitempty"`
	UsageType         UsageType      `json:"usage_type"`
	ReservedQuantity  float64        `json:"reserved_quantity,omitempty"`
	CommittedQuantity float64        `json:"committed_quantity,omitempty"`
	Unit              string         `json:"unit"`
	Source            string         `json:"source"`
	SourceID          string         `json:"source_id"`
	Status            string         `json:"status"`
	StartedAt         time.Time      `json:"started_at"`
	EndedAt           *time.Time     `json:"ended_at,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

type CommitUsageSessionInput struct {
	UserID      string         `json:"user_id"`
	CareGroupID string         `json:"care_group_id,omitempty"`
	CaredUserID string         `json:"cared_user_id,omitempty"`
	UsageType   UsageType      `json:"usage_type"`
	Quantity    float64        `json:"quantity,omitempty"`
	Unit        string         `json:"unit"`
	Source      string         `json:"source"`
	SourceID    string         `json:"source_id"`
	StartedAt   time.Time      `json:"started_at"`
	EndedAt     time.Time      `json:"ended_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UsageGrantInput struct {
	UserID      string         `json:"user_id"`
	CareGroupID string         `json:"care_group_id,omitempty"`
	CaredUserID string         `json:"cared_user_id,omitempty"`
	UsageType   UsageType      `json:"usage_type"`
	Quantity    float64        `json:"quantity"`
	Unit        string         `json:"unit"`
	Reason      string         `json:"reason"`
	Source      string         `json:"source"`
	SourceID    string         `json:"source_id"`
	GrantedByID string         `json:"granted_by_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UsageGrant struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id,omitempty"`
	UsageType   UsageType      `json:"usage_type"`
	Quantity    float64        `json:"quantity"`
	Unit        string         `json:"unit"`
	Reason      string         `json:"reason"`
	Source      string         `json:"source"`
	SourceID    string         `json:"source_id"`
	GrantedByID string         `json:"granted_by_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type UsageTotal struct {
	UsageType UsageType `json:"usage_type"`
	Quantity  float64   `json:"quantity"`
	Unit      string    `json:"unit"`
}

type UsageQuota struct {
	UsageType        UsageType `json:"usage_type"`
	FeatureKey       string    `json:"feature_key,omitempty"`
	Limit            float64   `json:"limit"`
	Used             float64   `json:"used"`
	Granted          float64   `json:"granted"`
	Remaining        float64   `json:"remaining"`
	Unit             string    `json:"unit,omitempty"`
	ResetPeriod      string    `json:"reset_period,omitempty"`
	Unlimited        bool      `json:"unlimited"`
	UpgradeSuggested bool      `json:"upgrade_suggested"`
}

type UsageResponse struct {
	Plan        PlanSummary              `json:"plan"`
	Quotas      map[UsageType]UsageQuota `json:"quotas"`
	TopUsage    []UsageTotal             `json:"top_usage"`
	PeriodStart time.Time                `json:"period_start"`
	PeriodEnd   time.Time                `json:"period_end"`
	AsOf        time.Time                `json:"as_of"`
}

type CareGroupSummary struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	OwnerUserID      string    `json:"owner_user_id,omitempty"`
	CurrentUserRole  string    `json:"current_user_role"`
	CanManageBilling bool      `json:"can_manage_billing"`
	CaredUserCount   int       `json:"cared_user_count"`
	MemberCount      int       `json:"member_count"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
}

type Plan struct {
	ID            string        `json:"id,omitempty"`
	Code          string        `json:"code"`
	Name          string        `json:"name"`
	PriceVND      int64         `json:"price_vnd"`
	BillingPeriod string        `json:"billing_period"`
	Status        string        `json:"status"`
	SortOrder     int32         `json:"sort_order"`
	Entitlements  []Entitlement `json:"entitlements"`
}

type PlanSummary struct {
	ID            string `json:"id,omitempty"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	PriceVND      int64  `json:"price_vnd"`
	BillingPeriod string `json:"billing_period"`
	Status        string `json:"status"`
}

type Entitlement struct {
	FeatureKey  string         `json:"feature_key"`
	LimitValue  any            `json:"limit_value"`
	LimitUnit   string         `json:"limit_unit,omitempty"`
	ResetPeriod string         `json:"reset_period,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type EntitlementValue struct {
	LimitValue  any            `json:"limit_value"`
	LimitUnit   string         `json:"limit_unit,omitempty"`
	ResetPeriod string         `json:"reset_period,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Subscription struct {
	ID                     string         `json:"id"`
	UserID                 string         `json:"user_id,omitempty"`
	CareGroupID            string         `json:"care_group_id,omitempty"`
	PlanID                 string         `json:"plan_id,omitempty"`
	PlanCode               string         `json:"plan_code"`
	PlanName               string         `json:"plan_name,omitempty"`
	Status                 string         `json:"status"`
	CurrentPeriodStart     *time.Time     `json:"current_period_start,omitempty"`
	CurrentPeriodEnd       *time.Time     `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool           `json:"cancel_at_period_end"`
	Provider               string         `json:"provider,omitempty"`
	ProviderSubscriptionID string         `json:"provider_subscription_id,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type SubscriptionResponse struct {
	Subscription  *Subscription `json:"subscription"`
	EffectivePlan PlanSummary   `json:"effective_plan"`
	AsOf          time.Time     `json:"as_of"`
}

type EntitlementsResponse struct {
	Plan         PlanSummary                 `json:"plan"`
	Entitlements map[string]EntitlementValue `json:"entitlements"`
	AsOf         time.Time                   `json:"as_of"`
}

type SubscriptionEvent struct {
	ID              string         `json:"id"`
	SubscriptionID  string         `json:"subscription_id,omitempty"`
	CareGroupID     string         `json:"care_group_id,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
	EventType       string         `json:"event_type"`
	Provider        string         `json:"provider,omitempty"`
	ProviderEventID string         `json:"provider_event_id,omitempty"`
	Payload         map[string]any `json:"-"`
	OccurredAt      time.Time      `json:"occurred_at"`
	CreatedAt       time.Time      `json:"created_at"`
}

type ProviderSubscriptionInput struct {
	UserID                 string         `json:"user_id"`
	CareGroupID            string         `json:"care_group_id,omitempty"`
	PlanID                 string         `json:"plan_id"`
	Status                 string         `json:"status"`
	CurrentPeriodStart     *time.Time     `json:"current_period_start,omitempty"`
	CurrentPeriodEnd       *time.Time     `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool           `json:"cancel_at_period_end"`
	Provider               string         `json:"provider"`
	ProviderSubscriptionID string         `json:"provider_subscription_id"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type SubscriptionEventInput struct {
	SubscriptionID  string         `json:"subscription_id,omitempty"`
	CareGroupID     string         `json:"care_group_id,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
	EventType       string         `json:"event_type"`
	Provider        string         `json:"provider,omitempty"`
	ProviderEventID string         `json:"provider_event_id,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
	OccurredAt      time.Time      `json:"occurred_at"`
}

type BillingTransaction struct {
	ID              string         `json:"id"`
	SubscriptionID  string         `json:"subscription_id,omitempty"`
	CareGroupID     string         `json:"care_group_id,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
	PlanID          string         `json:"plan_id,omitempty"`
	Provider        string         `json:"provider,omitempty"`
	ProviderEventID string         `json:"provider_event_id,omitempty"`
	Status          string         `json:"status"`
	Amount          float64        `json:"amount"`
	Currency        string         `json:"currency"`
	Payload         map[string]any `json:"-"`
	OccurredAt      time.Time      `json:"occurred_at"`
	CreatedAt       time.Time      `json:"created_at"`
}

type BillingTransactionInput struct {
	SubscriptionID  string         `json:"subscription_id,omitempty"`
	CareGroupID     string         `json:"care_group_id,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
	PlanID          string         `json:"plan_id,omitempty"`
	Provider        string         `json:"provider"`
	ProviderEventID string         `json:"provider_event_id"`
	Status          string         `json:"status"`
	Amount          float64        `json:"amount"`
	Currency        string         `json:"currency"`
	Payload         map[string]any `json:"payload,omitempty"`
	OccurredAt      time.Time      `json:"occurred_at"`
}

type StoreManagementInfo struct {
	Provider     string `json:"provider,omitempty"`
	ManageURL    string `json:"manage_url,omitempty"`
	Instructions string `json:"instructions"`
}

type BillingOverviewResponse struct {
	CareGroup       CareGroupSummary            `json:"care_group"`
	Subscription    *Subscription               `json:"subscription"`
	EffectivePlan   PlanSummary                 `json:"effective_plan"`
	Entitlements    map[string]EntitlementValue `json:"entitlements"`
	Usage           UsageResponse               `json:"usage"`
	Events          []SubscriptionEvent         `json:"events"`
	StoreManagement StoreManagementInfo         `json:"store_management"`
	AsOf            time.Time                   `json:"as_of"`
}

type BillingOverviewInput struct {
	UserID      string
	CareGroupID string
	CaredUserID string
}

func summarizePlan(plan Plan) PlanSummary {
	return PlanSummary{
		ID:            plan.ID,
		Code:          plan.Code,
		Name:          plan.Name,
		PriceVND:      plan.PriceVND,
		BillingPeriod: plan.BillingPeriod,
		Status:        plan.Status,
	}
}
