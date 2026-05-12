package tracking

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type PgxStore struct {
	db dbgen.DBTX
}

func NewPgxStore(db dbgen.DBTX) *PgxStore {
	return &PgxStore{db: db}
}

func (s *PgxStore) RecordEvent(ctx context.Context, input EventInput) (Event, error) {
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return Event{}, fmt.Errorf("marshal metadata: %w", err)
	}

	var actorID *string
	if input.ActorID != "" {
		actorID = &input.ActorID
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO tracking_events (
			user_id, actor_id, event_type, category, severity, source, source_id,
			status, duration_ms, occurred_at, metadata
		)
		VALUES (
			$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		RETURNING
			id::text, user_id::text, COALESCE(actor_id::text, ''), event_type,
			category, severity, source, source_id, status,
			COALESCE(duration_ms, 0), occurred_at, metadata
	`, input.UserID, actorID, input.EventType, input.Category, input.Severity,
		input.Source, input.SourceID, input.Status, input.DurationMS,
		input.OccurredAt, metadata)

	event, err := scanEvent(row)
	if err != nil {
		return Event{}, fmt.Errorf("record tracking event: %w", err)
	}
	return event, nil
}

func scanEvent(row pgx.Row) (Event, error) {
	var event Event
	var metadata []byte
	if err := row.Scan(
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
		return Event{}, err
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &event.Metadata)
	}
	return event, nil
}
