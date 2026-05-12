package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type Store struct {
	db dbgen.DBTX
}

func NewStore(db dbgen.DBTX) *Store {
	return &Store{db: db}
}

type UserResult struct {
	ID          string    `json:"id"`
	GoogleID    string    `json:"google_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	PlanCode    *string   `json:"plan_code,omitempty"`
	PlanName    *string   `json:"plan_name,omitempty"`
	SubStatus   *string   `json:"subscription_status,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserProfile struct {
	ID          string  `json:"id"`
	GoogleID    string  `json:"google_id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	IsVolunteer bool    `json:"is_volunteer"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type FamilyLinkResult struct {
	ID           string `json:"id"`
	FamilyUserID string `json:"family_user_id"`
	FamilyName   string `json:"family_name"`
	Relationship string `json:"relationship"`
	LinkedAt     string `json:"linked_at"`
}

type AdminSubscription struct {
	ID                     string  `json:"id"`
	PlanCode               string  `json:"plan_code"`
	PlanName               string  `json:"plan_name"`
	Status                 string  `json:"status"`
	CurrentPeriodStart     *string `json:"current_period_start,omitempty"`
	CurrentPeriodEnd       *string `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool    `json:"cancel_at_period_end"`
	Provider               *string `json:"provider,omitempty"`
	ProviderSubscriptionID *string `json:"provider_subscription_id,omitempty"`
}

type AdminUsageEntry struct {
	ID         string  `json:"id"`
	UsageType  string  `json:"usage_type"`
	Quantity   float64 `json:"quantity"`
	Unit       string  `json:"unit"`
	Source     string  `json:"source"`
	SourceID   string  `json:"source_id"`
	OccurredAt string  `json:"occurred_at"`
}

type AdminEntitlementResult struct {
	FeatureKey string `json:"feature_key"`
	LimitValue string `json:"limit_value"`
	LimitUnit  string `json:"limit_unit"`
}

type AdminSubEvent struct {
	ID              string `json:"id"`
	SubscriptionID  string `json:"subscription_id,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	CareGroupID     string `json:"care_group_id,omitempty"`
	EventType       string `json:"event_type"`
	Provider        string `json:"provider,omitempty"`
	ProviderEventID string `json:"provider_event_id,omitempty"`
	OccurredAt      string `json:"occurred_at"`
}

type AdminAnalytics struct {
	TotalUsers      int64            `json:"total_users"`
	UsersByRole     map[string]int64 `json:"users_by_role"`
	UsersByPlan     map[string]int64 `json:"users_by_plan"`
	RecentSignups   int64            `json:"recent_signups_7d"`
	ActiveSubs      int64            `json:"active_subscriptions"`
	TotalUsageTypes map[string]int64 `json:"total_usage_types"`
}

type TrackingSummary struct {
	WindowDays             int32            `json:"window_days"`
	AISessionsStarted      int64            `json:"ai_sessions_started"`
	AISessionsEnded        int64            `json:"ai_sessions_ended"`
	AISessionsFailed       int64            `json:"ai_sessions_failed"`
	AISuccessRate          float64          `json:"ai_success_rate"`
	AvgSessionDurationMS   float64          `json:"avg_session_duration_ms"`
	AvgFirstResponseMS     float64          `json:"avg_first_response_ms"`
	AIQuotaFallbacks       int64            `json:"ai_quota_fallbacks"`
	AIReconnects           int64            `json:"ai_reconnects"`
	AIToolCalls            int64            `json:"ai_tool_calls"`
	QuotaDenied            int64            `json:"quota_denied"`
	UsageRecorded          int64            `json:"usage_recorded"`
	UsageSessionsStarted   int64            `json:"usage_sessions_started"`
	UsageSessionsCommitted int64            `json:"usage_sessions_committed"`
	AICostRecorded         int64            `json:"ai_cost_recorded"`
	SafetyEvents           int64            `json:"safety_events"`
	CriticalSafetyEvents   int64            `json:"critical_safety_events"`
	MedicationTaken        int64            `json:"medication_taken"`
	MedicationSkipped      int64            `json:"medication_skipped"`
	FamilyAcknowledgements int64            `json:"family_acknowledgements"`
	AvgFamilyAckMS         float64          `json:"avg_family_ack_ms"`
	UnresolvedSafetyEvents int64            `json:"unresolved_safety_events"`
	AdminSupportEvents     int64            `json:"admin_support_events"`
	EventsByType           map[string]int64 `json:"events_by_type"`
	EventsByDay            []TrackingDay    `json:"events_by_day"`
	RecentEvents           []TrackingEvent  `json:"recent_events"`
}

type TrackingDay struct {
	Day    string `json:"day"`
	Events int64  `json:"events"`
}

type TrackingEvent struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	ActorID    string         `json:"actor_id,omitempty"`
	EventType  string         `json:"event_type"`
	Category   string         `json:"category"`
	Severity   string         `json:"severity"`
	Source     string         `json:"source"`
	SourceID   string         `json:"source_id"`
	Status     string         `json:"status"`
	DurationMS int64          `json:"duration_ms"`
	OccurredAt string         `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type UserTimelineEvent struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Title      string         `json:"title"`
	Category   string         `json:"category"`
	Severity   string         `json:"severity"`
	OccurredAt string         `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type AdminTrackingEventInput struct {
	UserID     string         `json:"user_id,omitempty"`
	EventType  string         `json:"event_type"`
	Source     string         `json:"source"`
	SourceID   string         `json:"source_id"`
	Status     string         `json:"status,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type AICostSummary struct {
	EventCount         int64              `json:"event_count"`
	EstimatedEvents    int64              `json:"estimated_events"`
	TotalCostUSD       float64            `json:"total_cost_usd"`
	TotalCostVND       float64            `json:"total_cost_vnd"`
	InputTokens        int64              `json:"input_tokens"`
	OutputTokens       int64              `json:"output_tokens"`
	AudioInputSeconds  float64            `json:"audio_input_seconds"`
	AudioOutputSeconds float64            `json:"audio_output_seconds"`
	CostByModel        map[string]float64 `json:"cost_by_model"`
	CostByDay          []AICostDayTotal   `json:"cost_by_day"`
	TopUsers           []AICostUserTotal  `json:"top_users"`
}

type AICostDayTotal struct {
	Day     string  `json:"day"`
	CostUSD float64 `json:"cost_usd"`
	CostVND float64 `json:"cost_vnd"`
	Events  int64   `json:"events"`
}

type AICostUserTotal struct {
	UserID      string  `json:"user_id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	CostUSD     float64 `json:"cost_usd"`
	CostVND     float64 `json:"cost_vnd"`
	Events      int64   `json:"events"`
}

type AICostEventResult struct {
	ID                 string         `json:"id"`
	Provider           string         `json:"provider"`
	Model              string         `json:"model"`
	Modality           string         `json:"modality"`
	Source             string         `json:"source"`
	SourceID           string         `json:"source_id"`
	InputTokens        int64          `json:"input_tokens"`
	OutputTokens       int64          `json:"output_tokens"`
	AudioInputSeconds  float64        `json:"audio_input_seconds"`
	AudioOutputSeconds float64        `json:"audio_output_seconds"`
	ImageCount         int64          `json:"image_count"`
	VideoSeconds       float64        `json:"video_seconds"`
	CostUSD            float64        `json:"cost_usd"`
	CostVND            float64        `json:"cost_vnd"`
	Currency           string         `json:"currency"`
	Estimated          bool           `json:"estimated"`
	PricingVersion     string         `json:"pricing_version"`
	OccurredAt         string         `json:"occurred_at"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type AdminBillingTransaction struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	UserEmail       string  `json:"user_email"`
	UserDisplayName string  `json:"user_display_name"`
	Provider        string  `json:"provider"`
	ProviderEventID string  `json:"provider_event_id"`
	Status          string  `json:"status"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	OccurredAt      string  `json:"occurred_at"`
}

func (s *Store) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM admin_users WHERE user_id = $1)
	`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("admin check: %w", err)
	}
	return exists, nil
}

func (s *Store) SearchUsers(ctx context.Context, query string, page, limit int32) ([]UserResult, error) {
	offset := (page - 1) * limit
	search := "%" + query + "%"

	rows, err := s.db.Query(ctx, `
		SELECT
			u.id::text,
			u.google_id,
			u.email,
			u.display_name,
			u.role,
			u.avatar_url,
			p.code,
			p.name,
			sub.status,
			u.created_at
		FROM users u
		LEFT JOIN LATERAL (
			SELECT s.plan_id, s.status
			FROM subscriptions s
			WHERE s.user_id = u.id
			  AND s.status IN ('trialing', 'active', 'past_due')
			ORDER BY s.created_at DESC
			LIMIT 1
		) sub ON true
		LEFT JOIN plans p ON p.id = sub.plan_id
		WHERE ($1 = '' OR u.email ILIKE $1 OR u.display_name ILIKE $1 OR u.google_id = $2)
		ORDER BY u.created_at DESC
		LIMIT $3 OFFSET $4
	`, search, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	var results []UserResult
	for rows.Next() {
		var r UserResult
		var avatarURL *string
		var planCode, planName, subStatus *string
		if err := rows.Scan(
			&r.ID, &r.GoogleID, &r.Email, &r.DisplayName, &r.Role,
			&avatarURL, &planCode, &planName, &subStatus, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		r.AvatarURL = avatarURL
		r.PlanCode = planCode
		r.PlanName = planName
		r.SubStatus = subStatus
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) GetUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	var p UserProfile
	var avatarURL *string
	err := s.db.QueryRow(ctx, `
		SELECT
			id::text,
			google_id,
			email,
			display_name,
			role,
			avatar_url,
			is_volunteer,
			created_at::text,
			updated_at::text
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&p.ID, &p.GoogleID, &p.Email, &p.DisplayName, &p.Role,
		&avatarURL, &p.IsVolunteer, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	p.AvatarURL = avatarURL
	return &p, nil
}

func (s *Store) GetFamilyLinks(ctx context.Context, userID string) ([]FamilyLinkResult, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			fl.id::text,
			COALESCE(fam.id::text, ''),
			COALESCE(fam.display_name, ''),
			fl.relationship,
			fl.created_at::text
		FROM family_links fl
		LEFT JOIN users fam ON (
			CASE WHEN fl.user_id = $1::uuid THEN fam.id = fl.family_id
			     ELSE fam.id = fl.user_id END
		)
		WHERE (fl.user_id = $1::uuid OR fl.family_id = $1::uuid)
		  AND fl.deleted_at IS NULL
		ORDER BY fl.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get family links: %w", err)
	}
	defer rows.Close()

	var results []FamilyLinkResult
	for rows.Next() {
		var r FamilyLinkResult
		if err := rows.Scan(&r.ID, &r.FamilyUserID, &r.FamilyName, &r.Relationship, &r.LinkedAt); err != nil {
			return nil, fmt.Errorf("scan family link: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) GetUserSubscription(ctx context.Context, userID string) (*AdminSubscription, error) {
	var sub AdminSubscription
	var start, end pgtype.Timestamptz
	var provider, providerSubID *string
	err := s.db.QueryRow(ctx, `
		SELECT
			s.id::text,
			p.code,
			p.name,
			s.status,
			s.current_period_start,
			s.current_period_end,
			s.cancel_at_period_end,
			s.provider,
			s.provider_subscription_id
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC
		LIMIT 1
	`, userID).Scan(
		&sub.ID, &sub.PlanCode, &sub.PlanName, &sub.Status,
		&start, &end, &sub.CancelAtPeriodEnd,
		&provider, &providerSubID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	if start.Valid {
		s := start.Time.Format(time.RFC3339)
		sub.CurrentPeriodStart = &s
	}
	if end.Valid {
		s := end.Time.Format(time.RFC3339)
		sub.CurrentPeriodEnd = &s
	}
	sub.Provider = provider
	sub.ProviderSubscriptionID = providerSubID
	return &sub, nil
}

func (s *Store) GetUserUsage(ctx context.Context, userID string, page, limit int32) ([]AdminUsageEntry, error) {
	offset := (page - 1) * limit
	rows, err := s.db.Query(ctx, `
		SELECT
			id::text,
			usage_type,
			quantity,
			unit,
			source,
			source_id,
			occurred_at::text
		FROM usage_ledger
		WHERE user_id = $1
		ORDER BY occurred_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	defer rows.Close()

	var results []AdminUsageEntry
	for rows.Next() {
		var r AdminUsageEntry
		if err := rows.Scan(
			&r.ID, &r.UsageType, &r.Quantity, &r.Unit,
			&r.Source, &r.SourceID, &r.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) GetUserEntitlements(ctx context.Context, userID string) ([]AdminEntitlementResult, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			pe.feature_key,
			pe.limit_value::text,
			COALESCE(pe.limit_unit, '')
		FROM subscriptions s
		JOIN plan_entitlements pe ON pe.plan_id = s.plan_id
		WHERE s.user_id = $1
		  AND s.status IN ('trialing', 'active', 'past_due')
		ORDER BY pe.feature_key
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get entitlements: %w", err)
	}
	defer rows.Close()

	var results []AdminEntitlementResult
	for rows.Next() {
		var r AdminEntitlementResult
		if err := rows.Scan(&r.FeatureKey, &r.LimitValue, &r.LimitUnit); err != nil {
			return nil, fmt.Errorf("scan entitlement: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) GetSubscriptionEvents(ctx context.Context, page, limit int32) ([]AdminSubEvent, error) {
	offset := (page - 1) * limit
	rows, err := s.db.Query(ctx, `
		SELECT
			id::text,
			COALESCE(subscription_id::text, ''),
			COALESCE(user_id::text, ''),
			COALESCE(care_group_id::text, ''),
			event_type,
			COALESCE(provider, ''),
			COALESCE(provider_event_id, ''),
			occurred_at::text
		FROM subscription_events
		ORDER BY occurred_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get subscription events: %w", err)
	}
	defer rows.Close()

	var results []AdminSubEvent
	for rows.Next() {
		var e AdminSubEvent
		var subID, userIDStr, careGroupID *string
		if err := rows.Scan(
			&e.ID, &subID, &userIDStr, &careGroupID,
			&e.EventType, &e.Provider, &e.ProviderEventID, &e.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan sub event: %w", err)
		}
		if subID != nil && *subID != "" {
			e.SubscriptionID = *subID
		}
		if userIDStr != nil && *userIDStr != "" {
			e.UserID = *userIDStr
		}
		if careGroupID != nil && *careGroupID != "" {
			e.CareGroupID = *careGroupID
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (s *Store) ListBillingTransactions(ctx context.Context, page, limit int32) ([]AdminBillingTransaction, error) {
	return s.listBillingTransactions(ctx, "", page, limit)
}

func (s *Store) ListUserBillingTransactions(ctx context.Context, userID string, page, limit int32) ([]AdminBillingTransaction, error) {
	return s.listBillingTransactions(ctx, userID, page, limit)
}

func (s *Store) listBillingTransactions(ctx context.Context, userID string, page, limit int32) ([]AdminBillingTransaction, error) {
	offset := (page - 1) * limit
	rows, err := s.db.Query(ctx, `
		SELECT
			bt.id::text,
			bt.user_id::text,
			u.email,
			u.display_name,
			bt.provider,
			bt.provider_event_id,
			bt.status,
			bt.amount::float8,
			bt.currency,
			bt.occurred_at::text
		FROM billing_transactions bt
		JOIN users u ON u.id = bt.user_id
		WHERE ($1 = '' OR bt.user_id = $1::uuid)
		ORDER BY bt.occurred_at DESC, bt.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list billing transactions: %w", err)
	}
	defer rows.Close()

	var transactions []AdminBillingTransaction
	for rows.Next() {
		var transaction AdminBillingTransaction
		if err := rows.Scan(
			&transaction.ID,
			&transaction.UserID,
			&transaction.UserEmail,
			&transaction.UserDisplayName,
			&transaction.Provider,
			&transaction.ProviderEventID,
			&transaction.Status,
			&transaction.Amount,
			&transaction.Currency,
			&transaction.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan billing transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate billing transactions: %w", err)
	}
	return transactions, nil
}

func (s *Store) GetAuditLogs(ctx context.Context, userID string, page, limit int32) ([]map[string]any, error) {
	offset := (page - 1) * limit
	rows, err := s.db.Query(ctx, `
		SELECT
			id::text,
			user_id::text,
			tool_name,
			params,
			result,
			created_at::text
		FROM audit_logs
		WHERE ($1 = '' OR user_id = $1::uuid)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get audit logs: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, uid, tool, createdAt string
		var params []byte
		var result *string
		if err := rows.Scan(&id, &uid, &tool, &params, &result, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		var paramsMap map[string]any
		if len(params) > 0 {
			_ = json.Unmarshal(params, &paramsMap)
		}
		if paramsMap == nil {
			paramsMap = map[string]any{}
		}
		entry := map[string]any{
			"id":         id,
			"user_id":    uid,
			"tool_name":  tool,
			"params":     paramsMap,
			"created_at": createdAt,
		}
		if result != nil {
			entry["result"] = *result
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (s *Store) GetAnalytics(ctx context.Context) (AdminAnalytics, error) {
	var a AdminAnalytics

	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&a.TotalUsers); err != nil {
		return a, fmt.Errorf("total users: %w", err)
	}

	roleRows, err := s.db.Query(ctx, `
		SELECT role, COUNT(*) FROM users GROUP BY role
	`)
	if err != nil {
		return a, fmt.Errorf("users by role: %w", err)
	}
	defer roleRows.Close()
	a.UsersByRole = map[string]int64{}
	for roleRows.Next() {
		var role string
		var count int64
		if err := roleRows.Scan(&role, &count); err != nil {
			return a, err
		}
		a.UsersByRole[role] = count
	}

	planRows, err := s.db.Query(ctx, `
		SELECT COALESCE(p.code, 'no_plan'), COUNT(*)
		FROM users u
		LEFT JOIN LATERAL (
			SELECT s.plan_id FROM subscriptions s
			WHERE s.user_id = u.id AND s.status IN ('trialing', 'active', 'past_due')
			ORDER BY s.created_at DESC LIMIT 1
		) sub ON true
		LEFT JOIN plans p ON p.id = sub.plan_id
		GROUP BY p.code
	`)
	if err != nil {
		return a, fmt.Errorf("users by plan: %w", err)
	}
	defer planRows.Close()
	a.UsersByPlan = map[string]int64{}
	for planRows.Next() {
		var plan string
		var count int64
		if err := planRows.Scan(&plan, &count); err != nil {
			return a, err
		}
		a.UsersByPlan[plan] = count
	}

	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE created_at > now() - INTERVAL '7 days'
	`).Scan(&a.RecentSignups); err != nil {
		return a, fmt.Errorf("recent signups: %w", err)
	}

	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM subscriptions WHERE status IN ('trialing', 'active', 'past_due')
	`).Scan(&a.ActiveSubs); err != nil {
		return a, fmt.Errorf("active subs: %w", err)
	}

	usageRows, err := s.db.Query(ctx, `
		SELECT usage_type, COUNT(*) FROM usage_ledger GROUP BY usage_type
	`)
	if err != nil {
		return a, fmt.Errorf("usage types: %w", err)
	}
	defer usageRows.Close()
	a.TotalUsageTypes = map[string]int64{}
	for usageRows.Next() {
		var t string
		var c int64
		if err := usageRows.Scan(&t, &c); err != nil {
			return a, err
		}
		a.TotalUsageTypes[t] = c
	}

	return a, nil
}

func (s *Store) GetAICostSummary(ctx context.Context, days int32) (AICostSummary, error) {
	if days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	var summary AICostSummary
	if err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COUNT(*) FILTER (WHERE estimated)::bigint,
			COALESCE(SUM(cost_usd), 0)::float8,
			COALESCE(SUM(cost_vnd), 0)::float8,
			COALESCE(SUM(input_tokens), 0)::bigint,
			COALESCE(SUM(output_tokens), 0)::bigint,
			COALESCE(SUM(audio_input_seconds), 0)::float8,
			COALESCE(SUM(audio_output_seconds), 0)::float8
		FROM ai_cost_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
	`, days).Scan(
		&summary.EventCount,
		&summary.EstimatedEvents,
		&summary.TotalCostUSD,
		&summary.TotalCostVND,
		&summary.InputTokens,
		&summary.OutputTokens,
		&summary.AudioInputSeconds,
		&summary.AudioOutputSeconds,
	); err != nil {
		return summary, fmt.Errorf("ai cost summary: %w", err)
	}

	modelRows, err := s.db.Query(ctx, `
		SELECT provider || ':' || model, COALESCE(SUM(cost_usd), 0)::float8
		FROM ai_cost_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
		GROUP BY provider, model
		ORDER BY SUM(cost_usd) DESC
	`, days)
	if err != nil {
		return summary, fmt.Errorf("ai cost by model: %w", err)
	}
	defer modelRows.Close()
	summary.CostByModel = map[string]float64{}
	for modelRows.Next() {
		var model string
		var cost float64
		if err := modelRows.Scan(&model, &cost); err != nil {
			return summary, err
		}
		summary.CostByModel[model] = cost
	}

	dayRows, err := s.db.Query(ctx, `
		SELECT
			date_trunc('day', occurred_at)::date::text,
			COALESCE(SUM(cost_usd), 0)::float8,
			COALESCE(SUM(cost_vnd), 0)::float8,
			COUNT(*)::bigint
		FROM ai_cost_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
		GROUP BY 1
		ORDER BY 1
	`, days)
	if err != nil {
		return summary, fmt.Errorf("ai cost by day: %w", err)
	}
	defer dayRows.Close()
	for dayRows.Next() {
		var row AICostDayTotal
		if err := dayRows.Scan(&row.Day, &row.CostUSD, &row.CostVND, &row.Events); err != nil {
			return summary, err
		}
		summary.CostByDay = append(summary.CostByDay, row)
	}

	topRows, err := s.db.Query(ctx, `
		SELECT
			e.user_id::text,
			COALESCE(u.email, ''),
			COALESCE(u.display_name, ''),
			COALESCE(SUM(e.cost_usd), 0)::float8,
			COALESCE(SUM(e.cost_vnd), 0)::float8,
			COUNT(*)::bigint
		FROM ai_cost_events e
		JOIN users u ON u.id = e.user_id
		WHERE e.occurred_at >= now() - ($1::int * INTERVAL '1 day')
		GROUP BY e.user_id, u.email, u.display_name
		ORDER BY SUM(e.cost_usd) DESC
		LIMIT 20
	`, days)
	if err != nil {
		return summary, fmt.Errorf("top ai cost users: %w", err)
	}
	defer topRows.Close()
	for topRows.Next() {
		var row AICostUserTotal
		if err := topRows.Scan(
			&row.UserID,
			&row.Email,
			&row.DisplayName,
			&row.CostUSD,
			&row.CostVND,
			&row.Events,
		); err != nil {
			return summary, err
		}
		summary.TopUsers = append(summary.TopUsers, row)
	}

	return summary, nil
}

func (s *Store) GetTrackingSummary(ctx context.Context, days int32) (TrackingSummary, error) {
	summary := TrackingSummary{
		WindowDays:   days,
		EventsByType: map[string]int64{},
		EventsByDay:  []TrackingDay{},
		RecentEvents: []TrackingEvent{},
	}
	if err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE event_type = 'ai_session_started')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'ai_session_ended')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'ai_session_failed')::bigint,
			COALESCE(AVG(duration_ms) FILTER (WHERE event_type = 'ai_session_ended'), 0)::float8,
			COALESCE(AVG(duration_ms) FILTER (WHERE event_type = 'ai_first_response'), 0)::float8,
			COUNT(*) FILTER (WHERE event_type = 'ai_quota_fallback')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'ai_reconnect')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'ai_tool_call')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'quota_check_denied')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'usage_recorded')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'usage_session_started')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'usage_session_committed')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'ai_cost_recorded')::bigint,
			COUNT(*) FILTER (WHERE category = 'safety_care')::bigint,
			COUNT(*) FILTER (WHERE category = 'safety_care' AND severity = 'critical')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'medication_taken')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'medication_skipped')::bigint,
			COUNT(*) FILTER (WHERE event_type = 'family_acknowledged')::bigint,
			COALESCE(AVG(duration_ms) FILTER (WHERE event_type = 'family_acknowledged'), 0)::float8,
			GREATEST(
				COUNT(*) FILTER (WHERE event_type IN ('emergency_triggered', 'family_notified')) -
				COUNT(*) FILTER (WHERE event_type = 'family_acknowledged'),
				0
			)::bigint,
			COUNT(*) FILTER (WHERE category = 'admin_support')::bigint
		FROM tracking_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
	`, days).Scan(
		&summary.AISessionsStarted,
		&summary.AISessionsEnded,
		&summary.AISessionsFailed,
		&summary.AvgSessionDurationMS,
		&summary.AvgFirstResponseMS,
		&summary.AIQuotaFallbacks,
		&summary.AIReconnects,
		&summary.AIToolCalls,
		&summary.QuotaDenied,
		&summary.UsageRecorded,
		&summary.UsageSessionsStarted,
		&summary.UsageSessionsCommitted,
		&summary.AICostRecorded,
		&summary.SafetyEvents,
		&summary.CriticalSafetyEvents,
		&summary.MedicationTaken,
		&summary.MedicationSkipped,
		&summary.FamilyAcknowledgements,
		&summary.AvgFamilyAckMS,
		&summary.UnresolvedSafetyEvents,
		&summary.AdminSupportEvents,
	); err != nil {
		return summary, fmt.Errorf("tracking summary: %w", err)
	}
	if summary.AISessionsStarted > 0 {
		summary.AISuccessRate = float64(summary.AISessionsEnded) / float64(summary.AISessionsStarted)
	}

	typeRows, err := s.db.Query(ctx, `
		SELECT event_type, COUNT(*)::bigint
		FROM tracking_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
		GROUP BY event_type
		ORDER BY COUNT(*) DESC, event_type
	`, days)
	if err != nil {
		return summary, fmt.Errorf("tracking events by type: %w", err)
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var eventType string
		var count int64
		if err := typeRows.Scan(&eventType, &count); err != nil {
			return summary, err
		}
		summary.EventsByType[eventType] = count
	}

	dayRows, err := s.db.Query(ctx, `
		SELECT date_trunc('day', occurred_at)::date::text, COUNT(*)::bigint
		FROM tracking_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
		GROUP BY 1
		ORDER BY 1
	`, days)
	if err != nil {
		return summary, fmt.Errorf("tracking events by day: %w", err)
	}
	defer dayRows.Close()
	for dayRows.Next() {
		var day TrackingDay
		if err := dayRows.Scan(&day.Day, &day.Events); err != nil {
			return summary, err
		}
		summary.EventsByDay = append(summary.EventsByDay, day)
	}

	eventRows, err := s.db.Query(ctx, `
		SELECT
			id::text, user_id::text, COALESCE(actor_id::text, ''), event_type,
			category, severity, source, source_id, status,
			COALESCE(duration_ms, 0), occurred_at::text, metadata
		FROM tracking_events
		WHERE occurred_at >= now() - ($1::int * INTERVAL '1 day')
		ORDER BY occurred_at DESC
		LIMIT 50
	`, days)
	if err != nil {
		return summary, fmt.Errorf("recent tracking events: %w", err)
	}
	defer eventRows.Close()
	for eventRows.Next() {
		event, err := scanTrackingEvent(eventRows)
		if err != nil {
			return summary, err
		}
		summary.RecentEvents = append(summary.RecentEvents, event)
	}

	return summary, nil
}

func (s *Store) GetUserTimeline(ctx context.Context, userID string, limit int32) ([]UserTimelineEvent, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, kind, title, category, severity, occurred_at, metadata
		FROM (
			SELECT
				id::text AS id,
				'tracking' AS kind,
				event_type AS title,
				category,
				severity,
				occurred_at::text AS occurred_at,
				metadata
			FROM tracking_events
			WHERE user_id = $1::uuid
			UNION ALL
			SELECT
				id::text AS id,
				'usage' AS kind,
				usage_type AS title,
				'usage' AS category,
				'info' AS severity,
				occurred_at::text AS occurred_at,
				jsonb_build_object(
					'quantity', quantity,
					'unit', unit,
					'source', source,
					'source_id', source_id
				) AS metadata
			FROM usage_ledger
			WHERE user_id = $1::uuid
			UNION ALL
			SELECT
				id::text AS id,
				'ai_cost' AS kind,
				provider || ':' || model AS title,
				'ai_ops' AS category,
				CASE WHEN estimated THEN 'warning' ELSE 'info' END AS severity,
				occurred_at::text AS occurred_at,
				jsonb_build_object(
					'cost_usd', cost_usd,
					'cost_vnd', cost_vnd,
					'input_tokens', input_tokens,
					'output_tokens', output_tokens,
					'estimated', estimated
				) AS metadata
			FROM ai_cost_events
			WHERE user_id = $1::uuid
			UNION ALL
			SELECT
				id::text AS id,
				'billing' AS kind,
				status AS title,
				'billing' AS category,
				CASE WHEN status IN ('failed', 'refunded') THEN 'warning' ELSE 'info' END AS severity,
				occurred_at::text AS occurred_at,
				jsonb_build_object(
					'provider', provider,
					'provider_event_id', provider_event_id,
					'amount', amount,
					'currency', currency
				) AS metadata
			FROM billing_transactions
			WHERE user_id = $1::uuid
			UNION ALL
			SELECT
				id::text AS id,
				'audit' AS kind,
				tool_name AS title,
				'audit' AS category,
				'info' AS severity,
				created_at::text AS occurred_at,
				params AS metadata
			FROM audit_logs
			WHERE user_id = $1::uuid
		) timeline
		ORDER BY occurred_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("user timeline: %w", err)
	}
	defer rows.Close()

	var events []UserTimelineEvent
	for rows.Next() {
		var event UserTimelineEvent
		var metadata []byte
		if err := rows.Scan(
			&event.ID,
			&event.Kind,
			&event.Title,
			&event.Category,
			&event.Severity,
			&event.OccurredAt,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan timeline event: %w", err)
		}
		event.Metadata = decodeJSONMap(metadata)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) RecordAdminTrackingEvent(
	ctx context.Context,
	actorID string,
	input AdminTrackingEventInput,
) (TrackingEvent, error) {
	if input.UserID == "" {
		input.UserID = actorID
	}
	if input.Source == "" {
		input.Source = "admin_dashboard"
	}
	if input.SourceID == "" {
		input.SourceID = input.EventType
	}
	if input.Status == "" {
		input.Status = "recorded"
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return TrackingEvent{}, fmt.Errorf("marshal tracking metadata: %w", err)
	}
	row := s.db.QueryRow(ctx, `
		INSERT INTO tracking_events (
			user_id, actor_id, event_type, category, severity, source, source_id,
			status, duration_ms, metadata
		)
		VALUES (
			$1::uuid, $2::uuid, $3, 'admin_support', 'info', $4, $5, $6, $7, $8
		)
		RETURNING
			id::text, user_id::text, COALESCE(actor_id::text, ''), event_type,
			category, severity, source, source_id, status,
			COALESCE(duration_ms, 0), occurred_at::text, metadata
	`, input.UserID, actorID, input.EventType, input.Source, input.SourceID,
		input.Status, input.DurationMS, metadata)
	event, err := scanTrackingEvent(row)
	if err != nil {
		return TrackingEvent{}, fmt.Errorf("record admin tracking event: %w", err)
	}
	return event, nil
}

type trackingScanner interface {
	Scan(dest ...any) error
}

func scanTrackingEvent(rows trackingScanner) (TrackingEvent, error) {
	var event TrackingEvent
	var metadata []byte
	if err := rows.Scan(
		&event.ID,
		&event.UserID,
		&event.ActorID,
		&event.EventType,
		&event.Category,
		&event.Severity,
		&event.Source,
		&event.SourceID,
		&event.Status,
		&event.DurationMS,
		&event.OccurredAt,
		&metadata,
	); err != nil {
		return TrackingEvent{}, fmt.Errorf("scan tracking event: %w", err)
	}
	event.Metadata = decodeJSONMap(metadata)
	return event, nil
}

func (s *Store) GetUserAICostEvents(ctx context.Context, userID string, limit int32) ([]AICostEventResult, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.Query(ctx, `
		SELECT
			id::text, provider, model, modality, source, source_id,
			input_tokens, output_tokens,
			audio_input_seconds::float8, audio_output_seconds::float8,
			image_count, video_seconds::float8, cost_usd::float8,
			cost_vnd::float8, currency, estimated, pricing_version,
			occurred_at::text, metadata
		FROM ai_cost_events
		WHERE user_id = $1::uuid
		ORDER BY occurred_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("user ai cost events: %w", err)
	}
	defer rows.Close()

	var events []AICostEventResult
	for rows.Next() {
		var event AICostEventResult
		var metadata []byte
		if err := rows.Scan(
			&event.ID,
			&event.Provider,
			&event.Model,
			&event.Modality,
			&event.Source,
			&event.SourceID,
			&event.InputTokens,
			&event.OutputTokens,
			&event.AudioInputSeconds,
			&event.AudioOutputSeconds,
			&event.ImageCount,
			&event.VideoSeconds,
			&event.CostUSD,
			&event.CostVND,
			&event.Currency,
			&event.Estimated,
			&event.PricingVersion,
			&event.OccurredAt,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan user ai cost event: %w", err)
		}
		event.Metadata = decodeJSONMap(metadata)
		events = append(events, event)
	}
	return events, rows.Err()
}

func decodeJSONMap(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	if len(value) == 0 {
		return nil
	}
	return value
}
