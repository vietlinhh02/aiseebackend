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

-- name: GetFamilyMembersWithDetails :many
SELECT u.id, u.google_id, u.email, u.display_name, u.role,
       u.is_volunteer, u.avatar_url, u.fcm_token, u.apns_token,
       u.apns_voip_token, u.settings, u.created_at, u.updated_at,
       fl.relationship
FROM users u
JOIN family_links fl ON (
    (fl.family_id = u.id AND fl.user_id = $1)
    OR
    (fl.user_id = u.id AND fl.family_id = $1)
)
WHERE fl.deleted_at IS NULL AND u.id != $1;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = COALESCE(NULLIF($2, ''), display_name),
    avatar_url = COALESCE($3, avatar_url),
    fcm_token = COALESCE($4, fcm_token),
    apns_token = COALESCE($5, apns_token),
    apns_voip_token = COALESCE($6, apns_voip_token),
    settings = COALESCE($7, settings),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ClearFCMToken :exec
UPDATE users SET fcm_token = NULL, updated_at = now() WHERE fcm_token = $1;

-- name: ClearAPNSToken :exec
UPDATE users SET apns_token = NULL, updated_at = now() WHERE apns_token = $1;

-- name: ClearAPNSVoIPToken :exec
UPDATE users SET apns_voip_token = NULL, updated_at = now() WHERE apns_voip_token = $1;

-- name: CheckFamilyLink :one
SELECT EXISTS(
    SELECT 1 FROM family_links
    WHERE ((user_id = $1 AND family_id = $2) OR (user_id = $2 AND family_id = $1))
      AND deleted_at IS NULL
) AS linked;
