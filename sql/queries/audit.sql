-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    actor_type,
    actor_id,
    action,
    entity_type,
    entity_id,
    payload,
    correlation_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAuditEventsByEntity :many
SELECT *
FROM audit_events
WHERE entity_type = $1
  AND entity_id = $2
ORDER BY occurred_at DESC
LIMIT $3;

-- name: ListAuditEvents :many
SELECT *
FROM audit_events
WHERE (sqlc.narg('entity_type')::text IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('entity_id')::uuid IS NULL OR entity_id = sqlc.narg('entity_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (
    sqlc.narg('cursor_time')::timestamptz IS NULL
    OR occurred_at < sqlc.narg('cursor_time')
    OR (occurred_at = sqlc.narg('cursor_time') AND id < sqlc.narg('cursor_id'))
  )
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg('limit_val');
