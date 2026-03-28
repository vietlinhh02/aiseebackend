-- name: GetMedicationLogsToday :many
-- All medication logs for today across all of a user's medications (ICT timezone)
SELECT ml.* FROM medication_logs ml
JOIN medications m ON m.id = ml.medication_id
WHERE m.user_id = $1
  AND ml.taken_at >= (now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date::timestamptz
  AND ml.taken_at < ((now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date + INTERVAL '1 day')::timestamptz;

-- name: GetRecentSymptoms :many
SELECT * FROM health_symptoms
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2;

-- name: GetRecentMetrics :many
SELECT * FROM health_metrics
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2;

-- name: UpdateUserLocation :exec
UPDATE users SET last_lat = $2, last_lng = $3, last_location_at = now()
WHERE id = $1;

-- name: GetUserLocation :one
SELECT last_lat, last_lng, last_location_at FROM users WHERE id = $1;
