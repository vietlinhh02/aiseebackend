package care

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGStore struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

func (s *PGStore) LoadWindow(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) (WindowData, error) {
	medications, err := s.loadMedications(ctx, userID)
	if err != nil {
		return WindowData{}, err
	}
	logs, err := s.loadMedicationLogs(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	symptoms, err := s.loadSymptoms(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	metrics, err := s.loadMetrics(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	audits, err := s.loadAudits(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	calls, err := s.loadCalls(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	careEv, err := s.loadCareEvents(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	navTrips, err := s.loadNavigationTrips(ctx, userID, start, end)
	if err != nil {
		return WindowData{}, err
	}
	return WindowData{
		Medications:    medications,
		MedicationLogs: logs,
		Symptoms:       symptoms,
		Metrics:        metrics,
		Audits:         audits,
		Calls:          calls,
		CareEvents:     careEv,
		NavigationTrips: navTrips,
	}, nil
}

func (s *PGStore) loadMedications(
	ctx context.Context,
	userID string,
) ([]MedicationInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, dosage, active
		FROM medications
		WHERE user_id = $1::uuid AND active = true
		ORDER BY lower(name), created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query medications: %w", err)
	}
	defer rows.Close()

	items := make([]MedicationInfo, 0)
	for rows.Next() {
		var item MedicationInfo
		if err := rows.Scan(&item.ID, &item.Name, &item.Dosage, &item.Active); err != nil {
			return nil, fmt.Errorf("scan medication: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate medications: %w", err)
	}
	return items, nil
}

func (s *PGStore) loadMedicationLogs(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]MedicationLogEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ml.medication_id::text, ml.taken_at, COALESCE(ml.skipped, false)
		FROM medication_logs ml
		JOIN medications m ON m.id = ml.medication_id
		WHERE m.user_id = $1::uuid
			AND ml.taken_at >= $2
			AND ml.taken_at < $3
		ORDER BY ml.taken_at DESC, ml.id DESC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query medication logs: %w", err)
	}
	defer rows.Close()

	items := make([]MedicationLogEntry, 0)
	for rows.Next() {
		var item MedicationLogEntry
		if err := rows.Scan(&item.MedicationID, &item.TakenAt, &item.Skipped); err != nil {
			return nil, fmt.Errorf("scan medication log: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate medication logs: %w", err)
	}
	return items, nil
}

func (s *PGStore) loadSymptoms(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]SymptomEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT symptom, COALESCE(severity, ''), COALESCE(notes, ''), logged_at
		FROM health_symptoms
		WHERE user_id = $1::uuid
			AND logged_at >= $2
			AND logged_at < $3
		ORDER BY logged_at DESC, symptom ASC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query symptoms: %w", err)
	}
	defer rows.Close()

	items := make([]SymptomEntry, 0)
	for rows.Next() {
		var item SymptomEntry
		if err := rows.Scan(&item.Symptom, &item.Severity, &item.Notes, &item.LoggedAt); err != nil {
			return nil, fmt.Errorf("scan symptom: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate symptoms: %w", err)
	}
	return items, nil
}

func (s *PGStore) loadMetrics(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]MetricEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT metric_type, value, COALESCE(unit, ''), logged_at
		FROM health_metrics
		WHERE user_id = $1::uuid
			AND logged_at >= $2
			AND logged_at < $3
		ORDER BY logged_at DESC, metric_type ASC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	items := make([]MetricEntry, 0)
	for rows.Next() {
		var item MetricEntry
		if err := rows.Scan(&item.MetricType, &item.Value, &item.Unit, &item.LoggedAt); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metrics: %w", err)
	}
	return items, nil
}

func (s *PGStore) loadAudits(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]AuditEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tool_name, params, COALESCE(result, ''), created_at
		FROM audit_logs
		WHERE user_id = $1::uuid
			AND created_at >= $2
			AND created_at < $3
			AND tool_name IN ('emergency_alert', 'escalate_to_family')
		ORDER BY created_at DESC, tool_name ASC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query audits: %w", err)
	}
	defer rows.Close()

	items := make([]AuditEntry, 0)
	for rows.Next() {
		var (
			item   AuditEntry
			params []byte
		)
		if err := rows.Scan(&item.ToolName, &params, &item.Result, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		if len(params) > 0 {
			_ = json.Unmarshal(params, &item.Params)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audits: %w", err)
	}
	return items, nil
}

func (s *PGStore) loadCalls(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]CallUsageEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT usage_type, quantity::float8, unit, source, source_id, occurred_at, metadata
		FROM usage_ledger
		WHERE user_id = $1::uuid
			AND occurred_at >= $2
			AND occurred_at < $3
			AND usage_type IN ('video_calls', 'video_call_minutes', 'voice_call_minutes')
		ORDER BY occurred_at DESC, usage_type ASC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query call usage: %w", err)
	}
	defer rows.Close()

	items := make([]CallUsageEntry, 0)
	for rows.Next() {
		var (
			item     CallUsageEntry
			metadata []byte
		)
		if err := rows.Scan(
			&item.UsageType,
			&item.Quantity,
			&item.Unit,
			&item.Source,
			&item.SourceID,
			&item.OccurredAt,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan call usage: %w", err)
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate call usage: %w", err)
	}
	return items, nil
}

// ── Navigation trips (in-app walking navigation history) ──

func (s *PGStore) loadNavigationTrips(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]NavigationTrip, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			id::text,
			destination_query,
			COALESCE(destination_name, ''),
			origin_lat, origin_lng,
			COALESCE(destination_lat, 0), COALESCE(destination_lng, 0),
			total_distance_m, total_duration_estimated_s,
			actual_duration_s, total_steps, steps_completed,
			deviation_count, outcome, mode,
			started_at, ended_at
		FROM navigation_history
		WHERE user_id = $1::uuid
			AND started_at >= $2
			AND started_at < $3
		ORDER BY started_at DESC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query navigation_history: %w", err)
	}
	defer rows.Close()

	items := make([]NavigationTrip, 0)
	for rows.Next() {
		var item NavigationTrip
		var endedAt *time.Time
		if err := rows.Scan(
			&item.ID,
			&item.DestinationQuery,
			&item.DestinationName,
			&item.OriginLat, &item.OriginLng,
			&item.DestinationLat, &item.DestinationLng,
			&item.TotalDistanceM, &item.TotalDurationEstimatedS,
			&item.ActualDurationS, &item.TotalSteps, &item.StepsCompleted,
			&item.DeviationCount, &item.Outcome, &item.Mode,
			&item.StartedAt, &endedAt,
		); err != nil {
			return nil, fmt.Errorf("scan navigation_history: %w", err)
		}
		if endedAt != nil {
			item.EndedAt = *endedAt
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate navigation_history: %w", err)
	}
	return items, nil
}
