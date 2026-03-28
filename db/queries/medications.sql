-- name: CreateMedication :one
INSERT INTO medications (user_id, name, dosage, times, frequency)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMedicationsByUser :many
SELECT * FROM medications
WHERE user_id = $1 AND active = true
ORDER BY name ASC;

-- name: GetMedicationByID :one
SELECT * FROM medications
WHERE id = $1 AND user_id = $2;

-- name: GetMedicationByName :many
SELECT * FROM medications
WHERE user_id = $1 AND active = true AND lower(name) = lower($2);

-- name: UpdateMedication :one
UPDATE medications
SET name = $3, dosage = $4, times = $5, frequency = $6
WHERE id = $1 AND user_id = $2 AND active = true
RETURNING *;

-- name: DeactivateMedication :exec
UPDATE medications
SET active = false, deleted_at = now()
WHERE id = $1 AND user_id = $2;

-- name: GetActiveMedicationsAll :many
SELECT * FROM medications WHERE active = true;

-- name: CreateMedicationLog :one
INSERT INTO medication_logs (medication_id, skipped)
VALUES ($1, $2)
RETURNING *;

-- name: GetMedicationLogsByDate :many
SELECT ml.* FROM medication_logs ml
JOIN medications m ON ml.medication_id = m.id
WHERE m.user_id = $1
  AND ml.taken_at >= $2::timestamptz
  AND ml.taken_at < $3::timestamptz
ORDER BY ml.taken_at ASC;

-- name: GetMedicationLogToday :many
SELECT ml.* FROM medication_logs ml
WHERE ml.medication_id = $1
  AND ml.taken_at >= (now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date::timestamptz
  AND ml.taken_at < ((now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date + INTERVAL '1 day')::timestamptz;
