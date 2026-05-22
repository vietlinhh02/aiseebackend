package care

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CareEventRow struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	ActorID    *string        `json:"actor_id,omitempty"`
	ActorRole  string         `json:"actor_role"`
	EventType  string         `json:"event_type"`
	Severity   string         `json:"severity"`
	Title      string         `json:"title"`
	Summary    *string        `json:"summary,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

type InsertCareEventInput struct {
	UserID     string
	ActorID    *string
	ActorRole  string
	EventType  string
	Severity   string
	Title      string
	Summary    *string
	Metadata   map[string]any
	OccurredAt time.Time
}

func (s *PGStore) InsertCareEvent(ctx context.Context, in InsertCareEventInput) error {
	meta := in.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("metadata json: %w", err)
	}
	sev := in.Severity
	if sev == "" {
		sev = "info"
	}
	role := in.ActorRole
	if role == "" {
		role = "system"
	}
	occ := in.OccurredAt
	if occ.IsZero() {
		occ = time.Now().UTC()
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO care_events (
			user_id, actor_id, actor_role, event_type, severity, title, summary, metadata, occurred_at
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9)
	`, in.UserID, in.ActorID, role, in.EventType, sev, in.Title, in.Summary, b, occ)
	if err != nil {
		return fmt.Errorf("insert care_event: %w", err)
	}
	return nil
}

func (s *PGStore) loadCareEvents(
	ctx context.Context,
	userID string,
	start, end time.Time,
) ([]CareEventRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, user_id::text, actor_id::text, actor_role, event_type, severity,
			title, summary, metadata, occurred_at, created_at
		FROM care_events
		WHERE user_id = $1::uuid AND occurred_at >= $2 AND occurred_at < $3
		ORDER BY occurred_at DESC, id DESC
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query care_events: %w", err)
	}
	defer rows.Close()
	return scanCareEventRows(rows)
}

func scanCareEventRows(rows pgx.Rows) ([]CareEventRow, error) {
	out := make([]CareEventRow, 0)
	for rows.Next() {
		var (
			r         CareEventRow
			actorID   *string
			summary   *string
			metaBytes []byte
		)
		if err := rows.Scan(
			&r.ID, &r.UserID, &actorID, &r.ActorRole, &r.EventType, &r.Severity,
			&r.Title, &summary, &metaBytes, &r.OccurredAt, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan care_event: %w", err)
		}
		r.ActorID = actorID
		r.Summary = summary
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &r.Metadata)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate care_events: %w", err)
	}
	return out, nil
}

func (s *PGStore) ListCareEventsForCitation(
	ctx context.Context,
	userID string,
	limit int,
) ([]CareEventRow, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, user_id::text, actor_id::text, actor_role, event_type, severity,
			title, summary, metadata, occurred_at, created_at
		FROM care_events
		WHERE user_id = $1::uuid
		ORDER BY occurred_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query care_events citation: %w", err)
	}
	defer rows.Close()
	return scanCareEventRows(rows)
}

type IncidentRow struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Status    string         `json:"status"`
	RoomName  *string        `json:"room_name,omitempty"`
	Lat       *float64       `json:"lat,omitempty"`
	Lng       *float64       `json:"lng,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	OpenedAt  time.Time      `json:"opened_at"`
	ClosedAt  *time.Time     `json:"closed_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type IncidentEventRow struct {
	ID         string         `json:"id"`
	IncidentID string         `json:"incident_id"`
	EventType  string         `json:"event_type"`
	ActorID    *string        `json:"actor_id,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
}

func (s *PGStore) CreateIncidentSOS(
	ctx context.Context,
	userID, roomName string,
	lat, lng float64,
	familyNotified int,
) (string, error) {
	meta, _ := json.Marshal(map[string]any{"family_notified": familyNotified})
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO incidents (user_id, status, room_name, lat, lng, metadata, opened_at)
		VALUES ($1::uuid, 'open', $2, $3, $4, $5::jsonb, now())
		RETURNING id::text
	`, userID, roomName, lat, lng, meta).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert incident: %w", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"room_name": roomName, "lat": lat, "lng": lng,
		"family_notified": familyNotified,
	})
	_, err = s.pool.Exec(ctx, `
		INSERT INTO incident_events (incident_id, event_type, payload, occurred_at)
		VALUES ($1::uuid, 'sos.triggered', $2::jsonb, now())
	`, id, payload)
	if err != nil {
		return "", fmt.Errorf("insert incident_event: %w", err)
	}
	return id, nil
}

func (s *PGStore) AppendIncidentEvent(
	ctx context.Context,
	incidentID, eventType string,
	actorID *string,
	payload map[string]any,
) error {
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO incident_events (incident_id, event_type, actor_id, payload, occurred_at)
		VALUES ($1::uuid, $2, $3::uuid, $4::jsonb, now())
	`, incidentID, eventType, actorID, b)
	if err != nil {
		return fmt.Errorf("append incident_event: %w", err)
	}
	return nil
}

func (s *PGStore) LatestIncidentInRange(
	ctx context.Context,
	userID string,
	start, end time.Time,
) (*IncidentRow, error) {
	var (
		r         IncidentRow
		room      *string
		lat, lng  *float64
		metaBytes []byte
		closed    *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text, status, room_name, lat, lng, metadata, opened_at, closed_at, created_at
		FROM incidents
		WHERE user_id = $1::uuid AND opened_at >= $2 AND opened_at < $3
		ORDER BY opened_at DESC
		LIMIT 1
	`, userID, start, end).Scan(
		&r.ID, &r.UserID, &r.Status, &room, &lat, &lng, &metaBytes, &r.OpenedAt, &closed, &r.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest incident: %w", err)
	}
	r.RoomName = room
	r.Lat = lat
	r.Lng = lng
	r.ClosedAt = closed
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &r.Metadata)
	}
	return &r, nil
}

func (s *PGStore) ListIncidentEvents(
	ctx context.Context,
	incidentID string,
) ([]IncidentEventRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, incident_id::text, event_type, actor_id::text, payload, occurred_at
		FROM incident_events
		WHERE incident_id = $1::uuid
		ORDER BY occurred_at ASC, id ASC
	`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("incident_events: %w", err)
	}
	defer rows.Close()
	out := make([]IncidentEventRow, 0)
	for rows.Next() {
		var (
			ev       IncidentEventRow
			actor    *string
			payloadB []byte
		)
		if err := rows.Scan(&ev.ID, &ev.IncidentID, &ev.EventType, &actor, &payloadB, &ev.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan incident_event: %w", err)
		}
		ev.ActorID = actor
		if len(payloadB) > 0 {
			_ = json.Unmarshal(payloadB, &ev.Payload)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *PGStore) GetLatestOpenIncidentID(ctx context.Context, userID string) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text FROM incidents
		WHERE user_id = $1::uuid AND status = 'open'
		ORDER BY opened_at DESC
		LIMIT 1
	`, userID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("latest open incident: %w", err)
	}
	return id, true, nil
}

func (s *PGStore) HasFamilyLinkBetween(ctx context.Context, userA, userB string) (bool, error) {
	var linked bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM family_links
			WHERE ((user_id = $1::uuid AND family_id = $2::uuid) OR (user_id = $2::uuid AND family_id = $1::uuid))
			  AND deleted_at IS NULL
		) AS linked
	`, userA, userB).Scan(&linked)
	if err != nil {
		return false, fmt.Errorf("family link check: %w", err)
	}
	return linked, nil
}

type CareAlertRow struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	RuleCode       string         `json:"rule_code"`
	Severity       string         `json:"severity"`
	Title          string         `json:"title"`
	Body           *string        `json:"body,omitempty"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	AcknowledgedBy *string        `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time     `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty"`
	DismissedAt    *time.Time     `json:"dismissed_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (s *PGStore) ListCareAlerts(
	ctx context.Context,
	userID string,
	statusFilter string,
	limit int,
) ([]CareAlertRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id::text, user_id::text, rule_code, severity, title, body, status, metadata,
				acknowledged_by::text, acknowledged_at, resolved_at, dismissed_at, created_at, updated_at
			FROM care_alerts
			WHERE user_id = $1::uuid AND status = $2
			ORDER BY created_at DESC
			LIMIT $3
		`, userID, statusFilter, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id::text, user_id::text, rule_code, severity, title, body, status, metadata,
				acknowledged_by::text, acknowledged_at, resolved_at, dismissed_at, created_at, updated_at
			FROM care_alerts
			WHERE user_id = $1::uuid
			ORDER BY created_at DESC
			LIMIT $2
		`, userID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()
	return scanCareAlertRows(rows)
}

func scanCareAlertRows(rows pgx.Rows) ([]CareAlertRow, error) {
	out := make([]CareAlertRow, 0)
	for rows.Next() {
		var (
			r         CareAlertRow
			body      *string
			ackBy     *string
			ackAt     *time.Time
			resAt     *time.Time
			disAt     *time.Time
			metaBytes []byte
		)
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.RuleCode, &r.Severity, &r.Title, &body, &r.Status, &metaBytes,
			&ackBy, &ackAt, &resAt, &disAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		r.Body = body
		r.AcknowledgedBy = ackBy
		r.AcknowledgedAt = ackAt
		r.ResolvedAt = resAt
		r.DismissedAt = disAt
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &r.Metadata)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGStore) InsertCareAlert(
	ctx context.Context,
	userID, ruleCode, severity, title string,
	body *string,
	metadata map[string]any,
) (string, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	b, _ := json.Marshal(metadata)
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO care_alerts (user_id, rule_code, severity, title, body, status, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5, 'open', $6::jsonb)
		RETURNING id::text
	`, userID, ruleCode, severity, title, body, b).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert alert: %w", err)
	}
	return id, nil
}

func (s *PGStore) UpdateCareAlertAck(
	ctx context.Context,
	alertID, familyUserID string,
) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE care_alerts
		SET status = 'acknowledged', acknowledged_by = $2::uuid, acknowledged_at = now(), updated_at = now()
		WHERE id = $1::uuid AND status = 'open'
	`, alertID, familyUserID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PGStore) UpdateCareAlertResolve(
	ctx context.Context,
	alertID, familyUserID string,
) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE care_alerts
		SET status = 'resolved', resolved_at = now(), updated_at = now(),
			acknowledged_by = COALESCE(acknowledged_by, $2::uuid),
			acknowledged_at = COALESCE(acknowledged_at, now())
		WHERE id = $1::uuid AND status IN ('open', 'acknowledged')
	`, alertID, familyUserID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PGStore) UpdateCareAlertDismiss(
	ctx context.Context,
	alertID, familyUserID string,
) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE care_alerts
		SET status = 'dismissed', dismissed_at = now(), updated_at = now(),
			acknowledged_by = COALESCE(acknowledged_by, $2::uuid)
		WHERE id = $1::uuid AND status IN ('open', 'acknowledged')
	`, alertID, familyUserID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PGStore) GetCareAlertForUser(
	ctx context.Context,
	alertID, userID string,
) (*CareAlertRow, error) {
	var (
		r         CareAlertRow
		body      *string
		ackBy     *string
		ackAt     *time.Time
		resAt     *time.Time
		disAt     *time.Time
		metaBytes []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text, rule_code, severity, title, body, status, metadata,
			acknowledged_by::text, acknowledged_at, resolved_at, dismissed_at, created_at, updated_at
		FROM care_alerts
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, alertID, userID).Scan(
		&r.ID, &r.UserID, &r.RuleCode, &r.Severity, &r.Title, &body, &r.Status, &metaBytes,
		&ackBy, &ackAt, &resAt, &disAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Body = body
	r.AcknowledgedBy = ackBy
	r.AcknowledgedAt = ackAt
	r.ResolvedAt = resAt
	r.DismissedAt = disAt
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &r.Metadata)
	}
	return &r, nil
}

func (s *PGStore) GetCareAlertByID(ctx context.Context, alertID string) (*CareAlertRow, error) {
	var (
		r         CareAlertRow
		body      *string
		ackBy     *string
		ackAt     *time.Time
		resAt     *time.Time
		disAt     *time.Time
		metaBytes []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text, rule_code, severity, title, body, status, metadata,
			acknowledged_by::text, acknowledged_at, resolved_at, dismissed_at, created_at, updated_at
		FROM care_alerts
		WHERE id = $1::uuid
	`, alertID).Scan(
		&r.ID, &r.UserID, &r.RuleCode, &r.Severity, &r.Title, &body, &r.Status, &metaBytes,
		&ackBy, &ackAt, &resAt, &disAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Body = body
	r.AcknowledgedBy = ackBy
	r.AcknowledgedAt = ackAt
	r.ResolvedAt = resAt
	r.DismissedAt = disAt
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &r.Metadata)
	}
	return &r, nil
}

type CareTaskRow struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	CreatedBy   string         `json:"created_by"`
	Title       string         `json:"title"`
	DueAt       *time.Time     `json:"due_at,omitempty"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

func (s *PGStore) ListCareTasks(ctx context.Context, userID string, limit int) ([]CareTaskRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, user_id::text, created_by::text, title, due_at, status, metadata,
			created_at, updated_at, completed_at
		FROM care_tasks
		WHERE user_id = $1::uuid
		ORDER BY
			CASE status WHEN 'open' THEN 0 ELSE 1 END,
			due_at NULLS LAST,
			created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	return scanCareTaskRows(rows)
}

func scanCareTaskRows(rows pgx.Rows) ([]CareTaskRow, error) {
	out := make([]CareTaskRow, 0)
	for rows.Next() {
		var (
			r         CareTaskRow
			due       *time.Time
			metaBytes []byte
			completed *time.Time
		)
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.CreatedBy, &r.Title, &due, &r.Status, &metaBytes,
			&r.CreatedAt, &r.UpdatedAt, &completed,
		); err != nil {
			return nil, err
		}
		r.DueAt = due
		r.CompletedAt = completed
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &r.Metadata)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGStore) InsertCareTask(
	ctx context.Context,
	userID, createdBy, title string,
	dueAt *time.Time,
	metadata map[string]any,
) (string, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	b, _ := json.Marshal(metadata)
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO care_tasks (user_id, created_by, title, due_at, status, metadata)
		VALUES ($1::uuid, $2::uuid, $3, $4, 'open', $5::jsonb)
		RETURNING id::text
	`, userID, createdBy, title, dueAt, b).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *PGStore) UpdateCareTask(
	ctx context.Context,
	taskID, userID, title string,
	dueAt *time.Time,
	status string,
	metadata map[string]any,
) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	b, _ := json.Marshal(metadata)
	cmd, err := s.pool.Exec(ctx, `
		UPDATE care_tasks
		SET title = $3, due_at = $4, status = COALESCE(NULLIF($5, ''), status),
			metadata = $6::jsonb, updated_at = now(),
			completed_at = CASE WHEN $5 = 'completed' AND completed_at IS NULL THEN now() ELSE completed_at END
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, taskID, userID, title, dueAt, status, b)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PGStore) CompleteCareTask(ctx context.Context, taskID, userID string) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE care_tasks
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid AND status = 'open'
	`, taskID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PGStore) GetCareTask(ctx context.Context, taskID, userID string) (*CareTaskRow, error) {
	var (
		r         CareTaskRow
		due       *time.Time
		metaBytes []byte
		completed *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text, created_by::text, title, due_at, status, metadata,
			created_at, updated_at, completed_at
		FROM care_tasks
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, taskID, userID).Scan(
		&r.ID, &r.UserID, &r.CreatedBy, &r.Title, &due, &r.Status, &metaBytes,
		&r.CreatedAt, &r.UpdatedAt, &completed,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.DueAt = due
	r.CompletedAt = completed
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &r.Metadata)
	}
	return &r, nil
}

func (s *PGStore) GetCareTaskByID(ctx context.Context, taskID string) (*CareTaskRow, error) {
	var (
		r         CareTaskRow
		due       *time.Time
		metaBytes []byte
		completed *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text, created_by::text, title, due_at, status, metadata,
			created_at, updated_at, completed_at
		FROM care_tasks
		WHERE id = $1::uuid
	`, taskID).Scan(
		&r.ID, &r.UserID, &r.CreatedBy, &r.Title, &due, &r.Status, &metaBytes,
		&r.CreatedAt, &r.UpdatedAt, &completed,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.DueAt = due
	r.CompletedAt = completed
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &r.Metadata)
	}
	return &r, nil
}

func (s *PGStore) GetMemberPermissions(
	ctx context.Context,
	caredUserID, familyMemberID string,
) (map[string]any, error) {
	var scopes []byte
	err := s.pool.QueryRow(ctx, `
		SELECT scopes FROM care_member_permissions
		WHERE cared_user_id = $1::uuid AND family_member_id = $2::uuid
	`, caredUserID, familyMemberID).Scan(&scopes)
	if err == pgx.ErrNoRows {
		return defaultPermissionScopes(), nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if len(scopes) > 0 {
		_ = json.Unmarshal(scopes, &m)
	}
	if m == nil {
		m = defaultPermissionScopes()
	}
	return m, nil
}

func defaultPermissionScopes() map[string]any {
	return map[string]any{
		"timeline": "read", "health": "read", "medications": "read",
		"tasks": "write", "exports": "read", "alerts": "write", "permissions": "none",
	}
}

func (s *PGStore) UpsertMemberPermissions(
	ctx context.Context,
	caredUserID, familyMemberID string,
	scopes map[string]any,
) error {
	if scopes == nil {
		scopes = defaultPermissionScopes()
	}
	b, err := json.Marshal(scopes)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO care_member_permissions (cared_user_id, family_member_id, scopes)
		VALUES ($1::uuid, $2::uuid, $3::jsonb)
		ON CONFLICT (cared_user_id, family_member_id) DO UPDATE SET
			scopes = EXCLUDED.scopes, updated_at = now()
	`, caredUserID, familyMemberID, b)
	return err
}

func (s *PGStore) InsertAuditLog(
	ctx context.Context,
	userID, toolName string,
	params map[string]any,
	result string,
) error {
	var paramsJSON []byte
	if params != nil {
		paramsJSON, _ = json.Marshal(params)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (user_id, tool_name, params, result)
		VALUES ($1::uuid, $2, $3::jsonb, $4)
	`, userID, toolName, paramsJSON, result)
	return err
}
