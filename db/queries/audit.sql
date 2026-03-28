-- name: CreateAuditLog :exec
INSERT INTO audit_logs (user_id, tool_name, params, result)
VALUES ($1, $2, $3, $4);

-- name: GetAuditLogsByUser :many
SELECT * FROM audit_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
