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
