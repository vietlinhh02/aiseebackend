-- name: CreateTrackingEvent :one
INSERT INTO tracking_events (
    user_id,
    actor_id,
    event_type,
    category,
    severity,
    source,
    source_id,
    status,
    duration_ms,
    occurred_at,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;
