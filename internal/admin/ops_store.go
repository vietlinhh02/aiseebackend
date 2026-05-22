package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminIncidentListItem struct {
	ID       string         `json:"id"`
	UserID   string         `json:"user_id"`
	Status   string         `json:"status"`
	OpenedAt string         `json:"opened_at"`
	ClosedAt *string        `json:"closed_at,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type AdminCareAlertListItem struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	RuleCode  string `json:"rule_code"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) ListIncidentsForAdmin(
	ctx context.Context,
	userID string,
	limit int32,
) ([]AdminIncidentListItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if userID != "" {
		rows, err = s.db.Query(ctx, `
			SELECT id::text, user_id::text, status, opened_at, closed_at, metadata
			FROM incidents
			WHERE user_id = $1::uuid
			ORDER BY opened_at DESC
			LIMIT $2
		`, userID, limit)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT id::text, user_id::text, status, opened_at, closed_at, metadata
			FROM incidents
			ORDER BY opened_at DESC
			LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()
	out := make([]AdminIncidentListItem, 0)
	for rows.Next() {
		var (
			item     AdminIncidentListItem
			meta     []byte
			closedAt *time.Time
			openedAt time.Time
		)
		if err := rows.Scan(&item.ID, &item.UserID, &item.Status, &openedAt, &closedAt, &meta); err != nil {
			return nil, err
		}
		item.OpenedAt = openedAt.UTC().Format(time.RFC3339)
		if closedAt != nil {
			s := closedAt.UTC().Format(time.RFC3339)
			item.ClosedAt = &s
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &item.Metadata)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListCareAlertsForAdmin(
	ctx context.Context,
	userID string,
	limit int32,
) ([]AdminCareAlertListItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if userID != "" {
		rows, err = s.db.Query(ctx, `
			SELECT id::text, user_id::text, rule_code, severity, title, status, created_at
			FROM care_alerts
			WHERE user_id = $1::uuid
			ORDER BY created_at DESC
			LIMIT $2
		`, userID, limit)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT id::text, user_id::text, rule_code, severity, title, status, created_at
			FROM care_alerts
			ORDER BY created_at DESC
			LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list care alerts: %w", err)
	}
	defer rows.Close()
	out := make([]AdminCareAlertListItem, 0)
	for rows.Next() {
		var (
			item AdminCareAlertListItem
			ts   time.Time
		)
		if err := rows.Scan(&item.ID, &item.UserID, &item.RuleCode, &item.Severity, &item.Title, &item.Status, &ts); err != nil {
			return nil, err
		}
		item.CreatedAt = ts.UTC().Format(time.RFC3339)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) InsertBillingRefundNote(
	ctx context.Context,
	billingTransactionID, adminUserID, note string,
) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO billing_refund_notes (billing_transaction_id, note, created_by_admin_id)
		VALUES ($1::uuid, $2, $3::uuid)
	`, billingTransactionID, note, adminUserID)
	if err != nil {
		return fmt.Errorf("insert refund note: %w", err)
	}
	return nil
}
