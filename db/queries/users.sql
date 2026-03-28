-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (google_id, email, display_name, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET display_name = $2, avatar_url = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateFCMToken :exec
UPDATE users SET fcm_token = $2, updated_at = now() WHERE id = $1;

-- name: UpdateAPNSToken :exec
UPDATE users SET apns_token = $2, updated_at = now() WHERE id = $1;

-- name: GetFamilyMembers :many
SELECT u.* FROM users u
JOIN family_links fl ON fl.family_id = u.id
WHERE fl.user_id = $1 AND fl.deleted_at IS NULL;

-- name: CreateFamilyLink :one
INSERT INTO family_links (user_id, family_id, relationship, is_primary_contact)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteFamilyLink :exec
UPDATE family_links SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
