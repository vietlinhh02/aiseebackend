-- name: CreateAppointment :one
INSERT INTO appointments (user_id, title, location, datetime, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAppointmentsByUser :many
SELECT * FROM appointments
WHERE user_id = $1 AND deleted_at IS NULL AND datetime >= now()
ORDER BY datetime ASC;

-- name: GetAppointmentByID :one
SELECT * FROM appointments
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: UpdateAppointment :one
UPDATE appointments
SET title = $3, location = $4, datetime = $5, notes = $6
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAppointment :exec
UPDATE appointments SET deleted_at = now()
WHERE id = $1 AND user_id = $2;

-- name: GetUpcomingAppointmentsAll :many
SELECT * FROM appointments
WHERE deleted_at IS NULL
  AND datetime >= now()
  AND datetime <= now() + INTERVAL '2 hours'
  AND (reminded_1h = false OR reminded_15m = false);

-- name: MarkAppointmentReminded1h :exec
UPDATE appointments SET reminded_1h = true WHERE id = $1;

-- name: MarkAppointmentReminded15m :exec
UPDATE appointments SET reminded_15m = true WHERE id = $1;
