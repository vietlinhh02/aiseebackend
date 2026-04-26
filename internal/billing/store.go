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
