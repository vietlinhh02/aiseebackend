-- name: CreateHealthSymptom :one
INSERT INTO health_symptoms (user_id, symptom, severity, notes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetHealthSymptomsByUser :many
SELECT * FROM health_symptoms
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateHealthMetric :one
INSERT INTO health_metrics (user_id, metric_type, value, unit)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetHealthMetricsByUser :many
SELECT * FROM health_metrics
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHealthMetricsByType :many
SELECT * FROM health_metrics
WHERE user_id = $1 AND metric_type = $2
ORDER BY logged_at DESC
LIMIT $3 OFFSET $4;
