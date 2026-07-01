-- name: AggregateSettlementJobStats :one
SELECT
    COUNT(*)::int AS total_job_count,
    COUNT(*) FILTER (WHERE status = 'pending')::int AS pending_count,
    COUNT(*) FILTER (WHERE status = 'processing')::int AS processing_count,
    COUNT(*) FILTER (WHERE status = 'failed')::int AS failed_count,
    COUNT(*) FILTER (WHERE status = 'dead_letter')::int AS dead_letter_count,
    COUNT(*) FILTER (WHERE status = 'succeeded')::int AS succeeded_count,
    COALESCE(SUM(net_payout_cents), 0)::bigint AS expected_total_cents,
    COALESCE(SUM(net_payout_cents) FILTER (WHERE status = 'succeeded'), 0)::bigint AS succeeded_total_cents
FROM settlement_jobs;

-- name: CreateReconciliationSnapshot :one
INSERT INTO reconciliation_snapshots (
    expected_job_count,
    expected_total_cents,
    succeeded_count,
    succeeded_total_cents,
    pending_count,
    failed_count,
    dead_letter_count,
    discrepancy_cents,
    details
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetLatestReconciliationSnapshot :one
SELECT *
FROM reconciliation_snapshots
ORDER BY snapshot_at DESC
LIMIT 1;
