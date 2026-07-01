-- name: GetActiveIdempotencyKey :one
SELECT *
FROM idempotency_keys
WHERE scope = $1
  AND key = $2
  AND expires_at > now();

-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (
    key,
    scope,
    request_hash,
    expires_at
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CompleteIdempotencyKey :one
UPDATE idempotency_keys
SET
    response_status = $3,
    response_body = $4
WHERE scope = $1
  AND key = $2
RETURNING *;
