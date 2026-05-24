package tracking

import "time"

type EventInput struct {
	UserID     string         `json:"user_id" validate:"required"`
	ActorID    string         `json:"actor_id,omitempty"`
	EventType  string         `json:"event_type" validate:"required"`
	Category   string         `json:"category" validate:"required"`
	Severity   string         `json:"severity,omitempty"`
	Source     string         `json:"source" validate:"required"`
	SourceID   string         `json:"source_id" validate:"required"`
	Status     string         `json:"status,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	OccurredAt time.Time      `json:"occurred_at,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Event struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	ActorID    string         `json:"actor_id,omitempty"`
	EventType  string         `json:"event_type"`
	Category   string         `json:"category"`
	Severity   string         `json:"severity"`
	Source     string         `json:"source"`
	SourceID   string         `json:"source_id"`
	Status     string         `json:"status,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
