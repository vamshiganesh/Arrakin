-- name: ListDueMaturitySchedules :many
-- Scheduler scan: pending maturities at or past due date.
-- FOR UPDATE SKIP LOCKED prevents duplicate enqueue under concurrent schedulers.
SELECT
    sqlc.embed(m),
    sqlc.embed(i)
FROM maturity_schedules m
INNER JOIN investments i ON i.id = m.investment_id
WHERE m.status = 'pending'
  AND m.matures_at <= now()
ORDER BY m.matures_at ASC
FOR UPDATE OF m SKIP LOCKED;

-- name: GetMaturityScheduleByID :one
SELECT *
FROM maturity_schedules
WHERE id = $1;

-- name: MarkMaturitySettled :one
UPDATE maturity_schedules
SET
    status = 'settled',
    settled_at = now()
WHERE id = $1
  AND status IN ('pending', 'processing')
RETURNING *;
