-- name: GetSettlementJobByID :one
SELECT *
FROM settlement_jobs
WHERE id = $1;

-- name: GetSettlementJobByMaturityScheduleID :one
SELECT *
FROM settlement_jobs
WHERE maturity_schedule_id = $1;

-- name: CreateSettlementJob :one
INSERT INTO settlement_jobs (
    maturity_schedule_id,
    investment_id,
    idempotency_key,
    status,
    principal_cents,
    gross_return_cents,
    platform_fee_cents,
    withholding_tax_cents,
    net_payout_cents,
    max_retries
)
VALUES (
    $1, $2, $3, 'pending',
    $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: ClaimSettlementJob :one
-- Atomically selects and leases the next claimable job for a worker.
WITH candidate AS (
    SELECT id
    FROM settlement_jobs
    WHERE status IN ('pending', 'failed')
      AND (next_retry_at IS NULL OR next_retry_at <= now())
    ORDER BY next_retry_at ASC NULLS FIRST, created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE settlement_jobs j
SET
    status = 'processing',
    processing_started_at = now(),
    processing_owner = $1,
    updated_at = now()
FROM candidate c
WHERE j.id = c.id
RETURNING j.*;

-- name: MarkJobSucceeded :one
UPDATE settlement_jobs
SET
    status = 'succeeded',
    payout_reference = $2,
    completed_at = now(),
    processing_started_at = NULL,
    processing_owner = NULL,
    last_error = NULL,
    error_class = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'processing'
RETURNING *;

-- name: MarkJobFailedRetryable :one
UPDATE settlement_jobs
SET
    status = 'failed',
    retry_count = retry_count + 1,
    next_retry_at = $2,
    last_error = $3,
    error_class = 'transient',
    processing_started_at = NULL,
    processing_owner = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'processing'
RETURNING *;

-- name: MarkJobDeadLetter :one
UPDATE settlement_jobs
SET
    status = 'dead_letter',
    dead_letter_reason = $2,
    last_error = $3,
    error_class = $4,
    processing_started_at = NULL,
    processing_owner = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'processing'
RETURNING *;

-- name: ReplayDeadLetterJob :one
UPDATE settlement_jobs
SET
    status = 'pending',
    dead_letter_reason = NULL,
    last_error = NULL,
    error_class = NULL,
    next_retry_at = NULL,
    processing_started_at = NULL,
    processing_owner = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'dead_letter'
RETURNING *;

-- name: ExpireStaleProcessingJobs :many
UPDATE settlement_jobs
SET
    status = 'pending',
    processing_started_at = NULL,
    processing_owner = NULL,
    updated_at = now()
WHERE status = 'processing'
  AND processing_started_at < $1
RETURNING *;
