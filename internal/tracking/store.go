package tracking

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
)

type PgxStore struct {
	queries *dbgen.Queries
}

func NewPgxStore(db dbgen.DBTX) *PgxStore {
	return &PgxStore{queries: dbgen.New(db)}
}

func (s *PgxStore) RecordEvent(ctx context.Context, input EventInput) (Event, error) {
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return Event{}, fmt.Errorf("marshal metadata: %w", err)
	}

	userID, err := pgutil.ParsePgUUID(input.UserID)
	if err != nil {
		return Event{}, fmt.Errorf("parse user_id: %w", err)
	}

	var actorID pgtype.UUID
	if input.ActorID != "" {
		actorID, err = pgutil.ParsePgUUID(input.ActorID)
		if err != nil {
			return Event{}, fmt.Errorf("parse actor_id: %w", err)
		}
	}

	durationMS := pgtype.Int8{}
	if input.DurationMS > 0 {
		durationMS = pgtype.Int8{Int64: input.DurationMS, Valid: true}
	}

	record, err := s.queries.CreateTrackingEvent(ctx, dbgen.CreateTrackingEventParams{
		UserID:     userID,
		ActorID:    actorID,
		EventType:  input.EventType,
		Category:   input.Category,
		Severity:   input.Severity,
		Source:     input.Source,
		SourceID:   input.SourceID,
		Status:     input.Status,
		DurationMs: durationMS,
		OccurredAt: pgtype.Timestamptz{Time: input.OccurredAt, Valid: true},
		Metadata:   metadata,
	})
	if err != nil {
		return Event{}, fmt.Errorf("record tracking event: %w", err)
	}
	return trackingEventFromDB(record), nil
}

func trackingEventFromDB(record dbgen.TrackingEvent) Event {
	event := Event{
		ID:         pgutil.PgUUIDToString(record.ID),
		UserID:     pgutil.PgUUIDToString(record.UserID),
		ActorID:    pgutil.PgUUIDToString(record.ActorID),
		EventType:  record.EventType,
		Category:   record.Category,
		Severity:   record.Severity,
		Source:     record.Source,
		SourceID:   record.SourceID,
		Status:     record.Status,
		OccurredAt: record.OccurredAt.Time.UTC(),
	}
	if record.DurationMs.Valid {
		event.DurationMS = record.DurationMs.Int64
	}
	if len(record.Metadata) > 0 {
		_ = json.Unmarshal(record.Metadata, &event.Metadata)
	}
	return event
}
