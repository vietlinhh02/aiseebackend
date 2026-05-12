package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type PgxStore struct {
	db dbgen.DBTX
}

func NewPgxStore(db dbgen.DBTX) *PgxStore {
	return &PgxStore{db: db}
}

func (s *PgxStore) ListPlans(ctx context.Context) ([]Plan, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, code, name, price_vnd, billing_period, status, sort_order
		FROM plans
		WHERE status = 'active'
		ORDER BY sort_order, price_vnd, code
	`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	var plans []Plan
	for rows.Next() {
		var plan Plan
		if err := rows.Scan(
			&plan.ID,
			&plan.Code,
			&plan.Name,
			&plan.PriceVND,
			&plan.BillingPeriod,
			&plan.Status,
			&plan.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plans: %w", err)
	}
	return plans, nil
}

func (s *PgxStore) DefaultPlan(ctx context.Context) (Plan, error) {
	return s.scanPlan(s.db.QueryRow(ctx, `
		SELECT id::text, code, name, price_vnd, billing_period, status, sort_order
		FROM plans
		WHERE status = 'active'
		ORDER BY sort_order, price_vnd, code
		LIMIT 1
	`))
}

func (s *PgxStore) DefaultCareGroup(
	ctx context.Context,
	userID string,
) (CareGroupSummary, error) {
	row := s.db.QueryRow(ctx, `
		WITH existing AS (
			SELECT
				cg.id,
				cg.name,
				cg.owner_user_id,
				cgm.role AS current_user_role,
				CASE cgm.role
					WHEN 'owner' THEN 0
					WHEN 'billing_admin' THEN 1
					ELSE 2
				END AS priority,
				(
					SELECT COUNT(*) FROM care_group_cared_users cgu
					WHERE cgu.care_group_id = cg.id AND cgu.deleted_at IS NULL
				)::int AS cared_user_count,
				(
					SELECT COUNT(*) FROM care_group_members cgm_count
					WHERE cgm_count.care_group_id = cg.id
					  AND cgm_count.deleted_at IS NULL
				)::int AS member_count
			FROM care_group_members cgm
			JOIN care_groups cg ON cg.id = cgm.care_group_id
			WHERE cgm.user_id = $1::uuid
			  AND cgm.deleted_at IS NULL
			  AND cg.status = 'active'

			UNION ALL

			SELECT
				cg.id,
				cg.name,
				cg.owner_user_id,
				'cared_user' AS current_user_role,
				10 AS priority,
				(
					SELECT COUNT(*) FROM care_group_cared_users cgu_count
					WHERE cgu_count.care_group_id = cg.id
					  AND cgu_count.deleted_at IS NULL
				)::int AS cared_user_count,
				(
					SELECT COUNT(*) FROM care_group_members cgm_count
					WHERE cgm_count.care_group_id = cg.id
					  AND cgm_count.deleted_at IS NULL
				)::int AS member_count
			FROM care_group_cared_users cgu
			JOIN care_groups cg ON cg.id = cgu.care_group_id
			WHERE cgu.user_id = $1::uuid
			  AND cgu.deleted_at IS NULL
			  AND cg.status = 'active'
			ORDER BY priority
			LIMIT 1
		),
		created AS (
			INSERT INTO care_groups (name, owner_user_id)
			SELECT 'AiSee Care Group', $1::uuid
			WHERE NOT EXISTS (SELECT 1 FROM existing)
			RETURNING id, name, owner_user_id
		),
		insert_member AS (
			INSERT INTO care_group_members (care_group_id, user_id, role)
			SELECT id, $1::uuid, 'owner' FROM created
			ON CONFLICT (care_group_id, user_id)
			DO UPDATE SET role = EXCLUDED.role, deleted_at = NULL
			RETURNING 1
		),
		insert_cared AS (
			INSERT INTO care_group_cared_users (care_group_id, user_id)
			SELECT id, $1::uuid FROM created
			ON CONFLICT (care_group_id, user_id)
			DO UPDATE SET deleted_at = NULL
			RETURNING 1
		),
		selected AS (
			SELECT
				id::text,
				name,
				owner_user_id::text,
				current_user_role,
				current_user_role IN ('owner', 'billing_admin') AS can_manage_billing,
				cared_user_count,
				member_count
			FROM existing

			UNION ALL

			SELECT
				id::text,
				name,
				owner_user_id::text,
				'owner',
				true,
				1,
				1
			FROM created
		)
		SELECT
			id, name, owner_user_id, current_user_role,
			can_manage_billing, cared_user_count, member_count
		FROM selected
		LIMIT 1
	`, userID)

	var group CareGroupSummary
	if err := row.Scan(
		&group.ID,
		&group.Name,
		&group.OwnerUserID,
		&group.CurrentUserRole,
		&group.CanManageBilling,
		&group.CaredUserCount,
		&group.MemberCount,
	); err != nil {
		return CareGroupSummary{}, fmt.Errorf("default care group: %w", err)
	}
	return group, nil
}

func (s *PgxStore) PlanByID(
	ctx context.Context,
	planID string,
) (Plan, error) {
	return s.scanPlan(s.db.QueryRow(ctx, `
		SELECT id::text, code, name, price_vnd, billing_period, status, sort_order
		FROM plans
		WHERE id = $1::uuid AND status = 'active'
	`, planID))
}

func (s *PgxStore) PlanByCode(
	ctx context.Context,
	code string,
) (Plan, error) {
	return s.scanPlan(s.db.QueryRow(ctx, `
		SELECT id::text, code, name, price_vnd, billing_period, status, sort_order
		FROM plans
		WHERE code = $1 AND status = 'active'
	`, code))
}

func (s *PgxStore) CurrentSubscription(
	ctx context.Context,
	userID string,
) (*Subscription, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			s.id::text,
			s.user_id::text,
			s.plan_id::text,
			p.code,
			p.name,
			s.status,
			s.current_period_start,
			s.current_period_end,
			s.cancel_at_period_end,
			COALESCE(s.provider, ''),
			COALESCE(s.provider_subscription_id, ''),
			COALESCE(s.metadata, '{}'::jsonb)
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.user_id = $1::uuid
		  AND s.status IN ('trialing', 'active', 'past_due')
		  AND p.status = 'active'
			ORDER BY
				CASE s.status
					WHEN 'active' THEN 1
					WHEN 'trialing' THEN 2
					ELSE 3
				END,
				p.sort_order DESC,
				p.price_vnd DESC,
				s.current_period_end DESC NULLS LAST,
				s.created_at DESC
			LIMIT 1
	`, userID)

	var sub Subscription
	var start, end pgtype.Timestamptz
	var metadata []byte
	if err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.PlanID,
		&sub.PlanCode,
		&sub.PlanName,
		&sub.Status,
		&start,
		&end,
		&sub.CancelAtPeriodEnd,
		&sub.Provider,
		&sub.ProviderSubscriptionID,
		&metadata,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("current subscription: %w", err)
	}
	sub.CurrentPeriodStart = timestamptzPtr(start)
	sub.CurrentPeriodEnd = timestamptzPtr(end)
	sub.Metadata = decodeJSONMap(metadata)
	return &sub, nil
}

func (s *PgxStore) CurrentSubscriptionForCareGroup(
	ctx context.Context,
	careGroupID string,
) (*Subscription, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			s.id::text,
			s.user_id::text,
			COALESCE(s.care_group_id::text, ''),
			s.plan_id::text,
			p.code,
			p.name,
			s.status,
			s.current_period_start,
			s.current_period_end,
			s.cancel_at_period_end,
			COALESCE(s.provider, ''),
			COALESCE(s.provider_subscription_id, ''),
			COALESCE(s.metadata, '{}'::jsonb)
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.care_group_id = $1::uuid
		  AND s.status IN ('trialing', 'active', 'past_due')
		  AND p.status = 'active'
			ORDER BY
				CASE s.status
					WHEN 'active' THEN 1
					WHEN 'trialing' THEN 2
					ELSE 3
				END,
				p.sort_order DESC,
				p.price_vnd DESC,
				s.current_period_end DESC NULLS LAST,
				s.created_at DESC
			LIMIT 1
	`, careGroupID)

	var sub Subscription
	var start, end pgtype.Timestamptz
	var metadata []byte
	if err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.CareGroupID,
		&sub.PlanID,
		&sub.PlanCode,
		&sub.PlanName,
		&sub.Status,
		&start,
		&end,
		&sub.CancelAtPeriodEnd,
		&sub.Provider,
		&sub.ProviderSubscriptionID,
		&metadata,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("current care group subscription: %w", err)
	}
	sub.CurrentPeriodStart = timestamptzPtr(start)
	sub.CurrentPeriodEnd = timestamptzPtr(end)
	sub.Metadata = decodeJSONMap(metadata)
	return &sub, nil
}

func (s *PgxStore) UpsertProviderSubscription(
	ctx context.Context,
	input ProviderSubscriptionInput,
) (Subscription, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return Subscription{}, fmt.Errorf("marshal subscription metadata: %w", err)
	}
	var careGroupID any
	if input.CareGroupID != "" {
		careGroupID = input.CareGroupID
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO subscriptions (
			user_id, care_group_id, plan_id, status,
			current_period_start, current_period_end,
			cancel_at_period_end, provider, provider_subscription_id, metadata
		)
		VALUES (
			$1::uuid, $2::uuid, $3::uuid, $4,
			$5, $6, $7, $8, $9, $10::jsonb
		)
		ON CONFLICT (provider, provider_subscription_id)
		WHERE provider IS NOT NULL AND provider_subscription_id IS NOT NULL
		DO UPDATE SET
			user_id = EXCLUDED.user_id,
			care_group_id = EXCLUDED.care_group_id,
			plan_id = EXCLUDED.plan_id,
			status = EXCLUDED.status,
			current_period_start = EXCLUDED.current_period_start,
			current_period_end = EXCLUDED.current_period_end,
			cancel_at_period_end = EXCLUDED.cancel_at_period_end,
			metadata = EXCLUDED.metadata,
			updated_at = now()
		RETURNING
			id::text,
			user_id::text,
			COALESCE(care_group_id::text, ''),
			plan_id::text,
			status,
			current_period_start,
			current_period_end,
			cancel_at_period_end,
			provider,
			provider_subscription_id,
			metadata
	`, input.UserID, careGroupID, input.PlanID, input.Status,
		input.CurrentPeriodStart, input.CurrentPeriodEnd,
		input.CancelAtPeriodEnd, input.Provider, input.ProviderSubscriptionID,
		metadata)

	var sub Subscription
	var start, end pgtype.Timestamptz
	var metadataJSON []byte
	if err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.CareGroupID,
		&sub.PlanID,
		&sub.Status,
		&start,
		&end,
		&sub.CancelAtPeriodEnd,
		&sub.Provider,
		&sub.ProviderSubscriptionID,
		&metadataJSON,
	); err != nil {
		return Subscription{}, fmt.Errorf("upsert provider subscription: %w", err)
	}
	plan, err := s.PlanByID(ctx, sub.PlanID)
	if err != nil {
		return Subscription{}, err
	}
	sub.PlanCode = plan.Code
	sub.PlanName = plan.Name
	sub.CurrentPeriodStart = timestamptzPtr(start)
	sub.CurrentPeriodEnd = timestamptzPtr(end)
	sub.Metadata = decodeJSONMap(metadataJSON)
	return sub, nil
}

func (s *PgxStore) InsertSubscriptionEvent(
	ctx context.Context,
	input SubscriptionEventInput,
) (SubscriptionEvent, error) {
	payload, err := marshalMetadata(input.Payload)
	if err != nil {
		return SubscriptionEvent{}, fmt.Errorf("marshal subscription event payload: %w", err)
	}
	var subscriptionID, careGroupID, userID any
	if input.SubscriptionID != "" {
		subscriptionID = input.SubscriptionID
	}
	if input.CareGroupID != "" {
		careGroupID = input.CareGroupID
	}
	if input.UserID != "" {
		userID = input.UserID
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO subscription_events (
			subscription_id, care_group_id, user_id, event_type,
			provider, provider_event_id, payload, occurred_at
		)
		VALUES (
			$1::uuid, $2::uuid, $3::uuid, $4,
			$5, $6, $7::jsonb, $8
		)
		ON CONFLICT DO NOTHING
		RETURNING
			id::text,
			COALESCE(subscription_id::text, ''),
			COALESCE(care_group_id::text, ''),
			COALESCE(user_id::text, ''),
			event_type,
			COALESCE(provider, ''),
			COALESCE(provider_event_id, ''),
			payload,
			occurred_at,
			created_at
	`, subscriptionID, careGroupID, userID, input.EventType,
		input.Provider, input.ProviderEventID, payload, input.OccurredAt)

	var event SubscriptionEvent
	var payloadJSON []byte
	if err := row.Scan(
		&event.ID,
		&event.SubscriptionID,
		&event.CareGroupID,
		&event.UserID,
		&event.EventType,
		&event.Provider,
		&event.ProviderEventID,
		&payloadJSON,
		&event.OccurredAt,
		&event.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return s.subscriptionEventByProviderEventID(
				ctx, input.Provider, input.ProviderEventID,
			)
		}
		return SubscriptionEvent{}, fmt.Errorf("insert subscription event: %w", err)
	}
	event.Payload = decodeJSONMap(payloadJSON)
	return event, nil
}

func (s *PgxStore) InsertBillingTransaction(
	ctx context.Context,
	input BillingTransactionInput,
) (BillingTransaction, error) {
	payload, err := marshalMetadata(input.Payload)
	if err != nil {
		return BillingTransaction{}, fmt.Errorf("marshal billing transaction payload: %w", err)
	}
	var subscriptionID, careGroupID, userID, planID any
	if input.SubscriptionID != "" {
		subscriptionID = input.SubscriptionID
	}
	if input.CareGroupID != "" {
		careGroupID = input.CareGroupID
	}
	if input.UserID != "" {
		userID = input.UserID
	}
	if input.PlanID != "" {
		planID = input.PlanID
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO billing_transactions (
			subscription_id, care_group_id, user_id, plan_id,
			provider, provider_event_id, status, amount,
			currency, occurred_at, payload
		)
		VALUES (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid,
			$5, $6, $7, $8, $9, $10, $11::jsonb
		)
		ON CONFLICT DO NOTHING
		RETURNING
			id::text,
			COALESCE(subscription_id::text, ''),
			COALESCE(care_group_id::text, ''),
			COALESCE(user_id::text, ''),
			COALESCE(plan_id::text, ''),
			provider,
			provider_event_id,
			status,
			amount::float8,
			currency,
			payload,
			occurred_at,
			created_at
	`, subscriptionID, careGroupID, userID, planID,
		input.Provider, input.ProviderEventID, input.Status, input.Amount,
		input.Currency, input.OccurredAt, payload)

	var tx BillingTransaction
	var payloadJSON []byte
	if err := row.Scan(
		&tx.ID,
		&tx.SubscriptionID,
		&tx.CareGroupID,
		&tx.UserID,
		&tx.PlanID,
		&tx.Provider,
		&tx.ProviderEventID,
		&tx.Status,
		&tx.Amount,
		&tx.Currency,
		&payloadJSON,
		&tx.OccurredAt,
		&tx.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return s.billingTransactionByProviderEventID(
				ctx, input.Provider, input.ProviderEventID,
			)
		}
		return BillingTransaction{}, fmt.Errorf("insert billing transaction: %w", err)
	}
	tx.Payload = decodeJSONMap(payloadJSON)
	return tx, nil
}

func (s *PgxStore) subscriptionEventByProviderEventID(
	ctx context.Context,
	provider string,
	providerEventID string,
) (SubscriptionEvent, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			id::text,
			COALESCE(subscription_id::text, ''),
			COALESCE(care_group_id::text, ''),
			COALESCE(user_id::text, ''),
			event_type,
			COALESCE(provider, ''),
			COALESCE(provider_event_id, ''),
			payload,
			occurred_at,
			created_at
		FROM subscription_events
		WHERE provider = $1 AND provider_event_id = $2
		LIMIT 1
	`, provider, providerEventID)

	var event SubscriptionEvent
	var payloadJSON []byte
	if err := row.Scan(
		&event.ID,
		&event.SubscriptionID,
		&event.CareGroupID,
		&event.UserID,
		&event.EventType,
		&event.Provider,
		&event.ProviderEventID,
		&payloadJSON,
		&event.OccurredAt,
		&event.CreatedAt,
	); err != nil {
		return SubscriptionEvent{}, fmt.Errorf("subscription event lookup: %w", err)
	}
	event.Payload = decodeJSONMap(payloadJSON)
	return event, nil
}

func (s *PgxStore) billingTransactionByProviderEventID(
	ctx context.Context,
	provider string,
	providerEventID string,
) (BillingTransaction, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			id::text,
			COALESCE(subscription_id::text, ''),
			COALESCE(care_group_id::text, ''),
			COALESCE(user_id::text, ''),
			COALESCE(plan_id::text, ''),
			provider,
			provider_event_id,
			status,
			amount::float8,
			currency,
			payload,
			occurred_at,
			created_at
		FROM billing_transactions
		WHERE provider = $1 AND provider_event_id = $2
		LIMIT 1
	`, provider, providerEventID)

	var tx BillingTransaction
	var payloadJSON []byte
	if err := row.Scan(
		&tx.ID,
		&tx.SubscriptionID,
		&tx.CareGroupID,
		&tx.UserID,
		&tx.PlanID,
		&tx.Provider,
		&tx.ProviderEventID,
		&tx.Status,
		&tx.Amount,
		&tx.Currency,
		&payloadJSON,
		&tx.OccurredAt,
		&tx.CreatedAt,
	); err != nil {
		return BillingTransaction{}, fmt.Errorf("billing transaction lookup: %w", err)
	}
	tx.Payload = decodeJSONMap(payloadJSON)
	return tx, nil
}

func (s *PgxStore) CaredUserIDsForCareGroup(
	ctx context.Context,
	careGroupID string,
) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id::text
		FROM care_group_cared_users
		WHERE care_group_id = $1::uuid
		  AND deleted_at IS NULL
		ORDER BY created_at, user_id
	`, careGroupID)
	if err != nil {
		return nil, fmt.Errorf("care group cared users: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan cared user id: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cared user ids: %w", err)
	}
	return userIDs, nil
}

func (s *PgxStore) SubscriptionEventsForCareGroup(
	ctx context.Context,
	careGroupID string,
	limit int,
) ([]SubscriptionEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(ctx, `
		SELECT
			id::text,
			COALESCE(subscription_id::text, ''),
			COALESCE(care_group_id::text, ''),
			COALESCE(user_id::text, ''),
			event_type,
			COALESCE(provider, ''),
			COALESCE(provider_event_id, ''),
			COALESCE(payload, '{}'::jsonb),
			occurred_at,
			created_at
		FROM subscription_events
		WHERE care_group_id = $1::uuid
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT $2
	`, careGroupID, limit)
	if err != nil {
		return nil, fmt.Errorf("care group subscription events: %w", err)
	}
	defer rows.Close()

	var events []SubscriptionEvent
	for rows.Next() {
		var event SubscriptionEvent
		var payload []byte
		if err := rows.Scan(
			&event.ID,
			&event.SubscriptionID,
			&event.CareGroupID,
			&event.UserID,
			&event.EventType,
			&event.Provider,
			&event.ProviderEventID,
			&payload,
			&event.OccurredAt,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription event: %w", err)
		}
		event.Payload = decodeJSONMap(payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription events: %w", err)
	}
	return events, nil
}

func (s *PgxStore) EntitlementsForPlan(
	ctx context.Context,
	planID string,
) ([]Entitlement, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			feature_key,
			limit_value,
			COALESCE(limit_unit, ''),
			COALESCE(reset_period, ''),
			COALESCE(metadata, '{}'::jsonb)
		FROM plan_entitlements
		WHERE plan_id = $1::uuid
		ORDER BY feature_key
	`, planID)
	if err != nil {
		return nil, fmt.Errorf("list entitlements: %w", err)
	}
	defer rows.Close()

	var entitlements []Entitlement
	for rows.Next() {
		var entitlement Entitlement
		var limitValue, metadata []byte
		if err := rows.Scan(
			&entitlement.FeatureKey,
			&limitValue,
			&entitlement.LimitUnit,
			&entitlement.ResetPeriod,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan entitlement: %w", err)
		}
		entitlement.LimitValue = decodeJSONAny(limitValue)
		entitlement.Metadata = decodeJSONMap(metadata)
		entitlements = append(entitlements, entitlement)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entitlements: %w", err)
	}
	return entitlements, nil
}

func (s *PgxStore) RecordUsage(
	ctx context.Context,
	input UsageRecordInput,
) (UsageRecord, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return UsageRecord{}, fmt.Errorf("marshal usage metadata: %w", err)
	}

	var record UsageRecord
	var metadataJSON []byte
	if err := s.db.QueryRow(ctx, `
		INSERT INTO usage_ledger (
			user_id, usage_type, quantity, unit, source, source_id,
			started_at, ended_at, metadata
		)
		VALUES (
			$1::uuid, $2, $3, $4, $5, $6,
			$7, $8, $9::jsonb
		)
		ON CONFLICT (user_id, source, source_id, usage_type)
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			unit = EXCLUDED.unit,
			started_at = EXCLUDED.started_at,
			ended_at = EXCLUDED.ended_at,
			metadata = EXCLUDED.metadata
		RETURNING
			id::text, user_id::text, usage_type, quantity::float8,
			unit, source, source_id, started_at, ended_at,
			metadata, created_at
	`, input.UserID, input.UsageType, input.Quantity, input.Unit,
		input.Source, input.SourceID, input.StartedAt, input.EndedAt,
		metadata).Scan(
		&record.ID,
		&record.UserID,
		&record.UsageType,
		&record.Quantity,
		&record.Unit,
		&record.Source,
		&record.SourceID,
		&record.StartedAt,
		&record.EndedAt,
		&metadataJSON,
		&record.CreatedAt,
	); err != nil {
		return UsageRecord{}, fmt.Errorf("record usage: %w", err)
	}
	record.Metadata = decodeJSONMap(metadataJSON)
	return record, nil
}

func (s *PgxStore) RecordAICostEvent(
	ctx context.Context,
	input AICostEventInput,
) (AICostEvent, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return AICostEvent{}, fmt.Errorf("marshal ai cost metadata: %w", err)
	}

	var event AICostEvent
	var metadataJSON []byte
	if err := s.db.QueryRow(ctx, `
		INSERT INTO ai_cost_events (
			user_id, usage_ledger_id, session_id, provider, model, modality,
			source, source_id, input_tokens, output_tokens,
			audio_input_seconds, audio_output_seconds, image_count,
			video_seconds, cost_usd, cost_vnd, currency, estimated,
			pricing_version, occurred_at, metadata
		)
		VALUES (
			$1::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid,
			$4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21::jsonb
		)
		ON CONFLICT (user_id, source, source_id, provider, model, modality, occurred_at)
		DO UPDATE SET
			usage_ledger_id = EXCLUDED.usage_ledger_id,
			session_id = EXCLUDED.session_id,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			audio_input_seconds = EXCLUDED.audio_input_seconds,
			audio_output_seconds = EXCLUDED.audio_output_seconds,
			image_count = EXCLUDED.image_count,
			video_seconds = EXCLUDED.video_seconds,
			cost_usd = EXCLUDED.cost_usd,
			cost_vnd = EXCLUDED.cost_vnd,
			currency = EXCLUDED.currency,
			estimated = EXCLUDED.estimated,
			pricing_version = EXCLUDED.pricing_version,
			metadata = EXCLUDED.metadata
		RETURNING
			id::text, user_id::text, COALESCE(usage_ledger_id::text, ''),
			COALESCE(session_id::text, ''), provider, model, modality,
			source, source_id, input_tokens, output_tokens,
			audio_input_seconds::float8, audio_output_seconds::float8,
			image_count, video_seconds::float8, cost_usd::float8,
			cost_vnd::float8, currency, estimated, pricing_version,
			occurred_at, metadata, created_at
	`, input.UserID, input.UsageLedgerID, input.SessionID,
		input.Provider, input.Model, input.Modality, input.Source,
		input.SourceID, input.InputTokens, input.OutputTokens,
		input.AudioInputSeconds, input.AudioOutputSeconds, input.ImageCount,
		input.VideoSeconds, input.CostUSD, input.CostVND, input.Currency,
		input.Estimated, input.PricingVersion, input.OccurredAt,
		metadata).Scan(
		&event.ID,
		&event.UserID,
		&event.UsageLedgerID,
		&event.SessionID,
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
		&metadataJSON,
		&event.CreatedAt,
	); err != nil {
		return AICostEvent{}, fmt.Errorf("record ai cost event: %w", err)
	}
	event.Metadata = decodeJSONMap(metadataJSON)
	return event, nil
}

func (s *PgxStore) StartUsageSession(
	ctx context.Context,
	input UsageSessionInput,
) (UsageSession, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return UsageSession{}, fmt.Errorf("marshal session metadata: %w", err)
	}

	var session UsageSession
	var metadataJSON []byte
	if err := s.db.QueryRow(ctx, `
		INSERT INTO usage_sessions (
			user_id, usage_type, reserved_quantity, unit, source,
			source_id, status, started_at, metadata
		)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, 'started', $7, $8::jsonb)
		ON CONFLICT (user_id, source, source_id, usage_type)
		DO UPDATE SET
			reserved_quantity = EXCLUDED.reserved_quantity,
			unit = EXCLUDED.unit,
			status = 'started',
			started_at = EXCLUDED.started_at,
			ended_at = NULL,
			metadata = EXCLUDED.metadata
		RETURNING
			id::text, user_id::text, usage_type,
			reserved_quantity::float8, committed_quantity::float8,
			unit, source, source_id, status, started_at, ended_at,
			metadata, created_at
	`, input.UserID, input.UsageType, input.ReservedQuantity,
		input.Unit, input.Source, input.SourceID, input.StartedAt,
		metadata).Scan(
		&session.ID,
		&session.UserID,
		&session.UsageType,
		&session.ReservedQuantity,
		&session.CommittedQuantity,
		&session.Unit,
		&session.Source,
		&session.SourceID,
		&session.Status,
		&session.StartedAt,
		&session.EndedAt,
		&metadataJSON,
		&session.CreatedAt,
	); err != nil {
		return UsageSession{}, fmt.Errorf("start usage session: %w", err)
	}
	session.Metadata = decodeJSONMap(metadataJSON)
	return session, nil
}

func (s *PgxStore) CommitUsageSession(
	ctx context.Context,
	input CommitUsageSessionInput,
) (UsageRecord, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return UsageRecord{}, fmt.Errorf("marshal session metadata: %w", err)
	}

	var record UsageRecord
	var metadataJSON []byte
	if err := s.db.QueryRow(ctx, `
		WITH session_row AS (
			INSERT INTO usage_sessions (
				user_id, usage_type, reserved_quantity, committed_quantity,
				unit, source, source_id, status, started_at, ended_at, metadata
			)
			VALUES (
				$1::uuid, $2, 0, $3, $4, $5, $6,
				'committed', $7, $8, $9::jsonb
			)
			ON CONFLICT (user_id, source, source_id, usage_type)
			DO UPDATE SET
				committed_quantity = EXCLUDED.committed_quantity,
				unit = EXCLUDED.unit,
				status = 'committed',
				started_at = EXCLUDED.started_at,
				ended_at = EXCLUDED.ended_at,
				metadata = EXCLUDED.metadata
			RETURNING id
		),
		ledger_row AS (
			INSERT INTO usage_ledger (
				session_id, user_id, usage_type, quantity, unit,
				source, source_id, started_at, ended_at, metadata
			)
			SELECT
				id, $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9::jsonb
			FROM session_row
			ON CONFLICT (user_id, source, source_id, usage_type)
			DO UPDATE SET
				session_id = EXCLUDED.session_id,
				quantity = EXCLUDED.quantity,
				unit = EXCLUDED.unit,
				started_at = EXCLUDED.started_at,
				ended_at = EXCLUDED.ended_at,
				metadata = EXCLUDED.metadata
			RETURNING
				id::text, user_id::text, usage_type, quantity::float8,
				unit, source, source_id, started_at, ended_at,
				metadata, created_at
		)
		SELECT * FROM ledger_row
	`, input.UserID, input.UsageType, input.Quantity, input.Unit,
		input.Source, input.SourceID, input.StartedAt, input.EndedAt,
		metadata).Scan(
		&record.ID,
		&record.UserID,
		&record.UsageType,
		&record.Quantity,
		&record.Unit,
		&record.Source,
		&record.SourceID,
		&record.StartedAt,
		&record.EndedAt,
		&metadataJSON,
		&record.CreatedAt,
	); err != nil {
		return UsageRecord{}, fmt.Errorf("commit usage session: %w", err)
	}
	record.Metadata = decodeJSONMap(metadataJSON)
	return record, nil
}

func (s *PgxStore) GrantUsageQuota(
	ctx context.Context,
	input UsageGrantInput,
) (UsageGrant, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return UsageGrant{}, fmt.Errorf("marshal grant metadata: %w", err)
	}

	var grantedBy any
	if input.GrantedByID != "" {
		grantedBy = input.GrantedByID
	}

	var grant UsageGrant
	var metadataJSON []byte
	if err := s.db.QueryRow(ctx, `
		INSERT INTO usage_quota_grants (
			user_id, usage_type, quantity, unit, reason, source,
			source_id, granted_by_id, metadata
		)
		VALUES (
			$1::uuid, $2, $3, $4, $5, $6, $7, $8::uuid, $9::jsonb
		)
		ON CONFLICT (user_id, source, source_id, usage_type)
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			unit = EXCLUDED.unit,
			reason = EXCLUDED.reason,
			granted_by_id = EXCLUDED.granted_by_id,
			metadata = EXCLUDED.metadata
		RETURNING
			id::text, user_id::text, usage_type, quantity::float8,
			unit, reason, source, source_id,
			COALESCE(granted_by_id::text, ''), metadata, created_at
	`, input.UserID, input.UsageType, input.Quantity, input.Unit,
		input.Reason, input.Source, input.SourceID, grantedBy,
		metadata).Scan(
		&grant.ID,
		&grant.UserID,
		&grant.UsageType,
		&grant.Quantity,
		&grant.Unit,
		&grant.Reason,
		&grant.Source,
		&grant.SourceID,
		&grant.GrantedByID,
		&metadataJSON,
		&grant.CreatedAt,
	); err != nil {
		return UsageGrant{}, fmt.Errorf("grant usage quota: %w", err)
	}
	grant.Metadata = decodeJSONMap(metadataJSON)
	return grant, nil
}

func (s *PgxStore) UsageTotals(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]UsageTotal, error) {
	rows, err := s.db.Query(ctx, `
		SELECT usage_type, COALESCE(SUM(quantity), 0)::float8, unit
		FROM usage_ledger
		WHERE user_id = $1::uuid
		  AND occurred_at >= $2
		  AND occurred_at < $3
		GROUP BY usage_type, unit
		ORDER BY SUM(quantity) DESC, usage_type
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("usage totals: %w", err)
	}
	defer rows.Close()
	return scanUsageTotals(rows)
}

func (s *PgxStore) UsageGrantTotals(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]UsageTotal, error) {
	rows, err := s.db.Query(ctx, `
		SELECT usage_type, COALESCE(SUM(quantity), 0)::float8, unit
		FROM usage_quota_grants
		WHERE user_id = $1::uuid
		  AND created_at >= $2
		  AND created_at < $3
		GROUP BY usage_type, unit
		ORDER BY usage_type
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("usage grant totals: %w", err)
	}
	defer rows.Close()
	return scanUsageTotals(rows)
}

func (s *PgxStore) UsageTotalsForUsers(
	ctx context.Context,
	userIDs []string,
	start time.Time,
	end time.Time,
) ([]UsageTotal, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT usage_type, COALESCE(SUM(quantity), 0)::float8, unit
		FROM usage_ledger
		WHERE user_id::text = ANY($1)
		  AND occurred_at >= $2
		  AND occurred_at < $3
		GROUP BY usage_type, unit
		ORDER BY SUM(quantity) DESC, usage_type
	`, userIDs, start, end)
	if err != nil {
		return nil, fmt.Errorf("usage totals for users: %w", err)
	}
	defer rows.Close()
	return scanUsageTotals(rows)
}

func (s *PgxStore) UsageGrantTotalsForUsers(
	ctx context.Context,
	userIDs []string,
	start time.Time,
	end time.Time,
) ([]UsageTotal, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT usage_type, COALESCE(SUM(quantity), 0)::float8, unit
		FROM usage_quota_grants
		WHERE user_id::text = ANY($1)
		  AND created_at >= $2
		  AND created_at < $3
		GROUP BY usage_type, unit
		ORDER BY usage_type
	`, userIDs, start, end)
	if err != nil {
		return nil, fmt.Errorf("usage grant totals for users: %w", err)
	}
	defer rows.Close()
	return scanUsageTotals(rows)
}

func (s *PgxStore) scanPlan(row pgx.Row) (Plan, error) {
	var plan Plan
	if err := row.Scan(
		&plan.ID,
		&plan.Code,
		&plan.Name,
		&plan.PriceVND,
		&plan.BillingPeriod,
		&plan.Status,
		&plan.SortOrder,
	); err != nil {
		return Plan{}, fmt.Errorf("scan plan: %w", err)
	}
	return plan, nil
}

func scanUsageTotals(rows pgx.Rows) ([]UsageTotal, error) {
	var totals []UsageTotal
	for rows.Next() {
		var total UsageTotal
		if err := rows.Scan(
			&total.UsageType,
			&total.Quantity,
			&total.Unit,
		); err != nil {
			return nil, fmt.Errorf("scan usage total: %w", err)
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage totals: %w", err)
	}
	return totals, nil
}

func marshalMetadata(metadata map[string]any) ([]byte, error) {
	if metadata == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(metadata)
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func decodeJSONAny(data []byte) any {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	return value
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
