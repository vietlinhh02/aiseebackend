-- name: CreateVolunteer :one
INSERT INTO volunteers (user_id, languages)
VALUES ($1, $2)
RETURNING *;

-- name: GetVolunteerByUserID :one
SELECT * FROM volunteers WHERE user_id = $1;

-- name: UpdateVolunteerAvailability :exec
UPDATE volunteers SET available = $1 WHERE user_id = $2;

-- name: GetAvailableVolunteers :many
SELECT v.*, u.display_name, u.fcm_token, u.apns_token, u.apns_voip_token, u.settings
FROM volunteers v
JOIN users u ON u.id = v.user_id
WHERE v.available = true
  AND v.user_id != $1
  AND v.user_id NOT IN (
    SELECT family_id FROM family_links WHERE user_id = $1 AND deleted_at IS NULL
    UNION
    SELECT user_id FROM family_links WHERE family_id = $1 AND deleted_at IS NULL
  )
ORDER BY v.rating DESC, v.total_calls ASC
LIMIT 5;

-- name: CreateVolunteerRating :exec
INSERT INTO volunteer_ratings (volunteer_id, rater_id, match_id, score)
VALUES ($1, $2, $3, $4)
ON CONFLICT (match_id, rater_id) DO NOTHING;

-- name: IncrementVolunteerCalls :exec
UPDATE volunteers SET total_calls = total_calls + 1 WHERE user_id = $1;

-- name: RecalcVolunteerRating :exec
UPDATE volunteers SET
    rating = COALESCE((SELECT AVG(score)::REAL FROM volunteer_ratings WHERE volunteer_id = $1), 0)
WHERE user_id = $1;
