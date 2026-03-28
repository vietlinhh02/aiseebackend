-- name: CreateMessage :one
INSERT INTO messages (sender_id, receiver_id, content_text, voice_url)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMessagesByContact :many
SELECT * FROM messages
WHERE ((sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1))
ORDER BY created_at DESC
LIMIT $3;

-- name: GetMessagesByContactBefore :many
SELECT m.* FROM messages m
WHERE ((m.sender_id = $1 AND m.receiver_id = $2) OR (m.sender_id = $2 AND m.receiver_id = $1))
  AND m.created_at < (SELECT cursor.created_at FROM messages cursor WHERE cursor.id = $3)
ORDER BY m.created_at DESC
LIMIT $4;

-- name: GetUnreadMessages :many
SELECT * FROM messages
WHERE receiver_id = $1 AND is_read = false
ORDER BY created_at ASC;

-- name: GetUnreadMessagesByContact :many
SELECT * FROM messages
WHERE receiver_id = $1 AND sender_id = $2 AND is_read = false
ORDER BY created_at ASC;

-- name: MarkMessageRead :exec
UPDATE messages SET is_read = true WHERE id = $1 AND receiver_id = $2;

-- name: CountUnreadMessages :one
SELECT count(*) FROM messages WHERE receiver_id = $1 AND is_read = false;
