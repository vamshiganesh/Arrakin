-- name: CreatePayoutAttempt :one
INSERT INTO payout_attempts (
    settlement_job_id,
    attempt_number,
    status
)
VALUES ($1, $2, 'started')
RETURNING *;

-- name: FinishPayoutAttemptSuccess :one
UPDATE payout_attempts
SET
    status = 'succeeded',
    payout_reference = $2,
    finished_at = now()
WHERE id = $1
  AND status = 'started'
RETURNING *;

-- name: FinishPayoutAttemptFailure :one
UPDATE payout_attempts
SET
    status = 'failed',
    error_message = $2,
    error_class = $3,
    finished_at = now()
WHERE id = $1
  AND status = 'started'
RETURNING *;

-- name: GetNextAttemptNumber :one
SELECT COALESCE(MAX(attempt_number), 0) + 1 AS next_attempt_number
FROM payout_attempts
WHERE settlement_job_id = $1;

-- name: ListPayoutAttemptsByJobID :many
SELECT *
FROM payout_attempts
WHERE settlement_job_id = $1
ORDER BY attempt_number ASC;
