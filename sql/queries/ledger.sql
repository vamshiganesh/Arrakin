-- name: UpsertLedgerAccount :one
INSERT INTO ledger_accounts (code, name, account_type)
VALUES ($1, $2, $3)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name
RETURNING *;

-- name: GetLedgerAccountByCode :one
SELECT *
FROM ledger_accounts
WHERE code = $1;

-- name: InsertLedgerEntry :one
INSERT INTO ledger_entries (
    entry_group_id,
    settlement_job_id,
    account_id,
    side,
    amount_cents,
    currency,
    description,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetLedgerEntryGroupIDByJobID :one
SELECT entry_group_id
FROM ledger_entries
WHERE settlement_job_id = $1
LIMIT 1;

-- name: ListLedgerEntriesByJobID :many
SELECT *
FROM ledger_entries
WHERE settlement_job_id = $1
ORDER BY posted_at ASC, side DESC;

-- name: ListLedgerEntries :many
SELECT le.*
FROM ledger_entries le
JOIN ledger_accounts la ON la.id = le.account_id
WHERE (sqlc.narg('settlement_job_id')::uuid IS NULL OR le.settlement_job_id = sqlc.narg('settlement_job_id'))
  AND (sqlc.narg('account_code')::text IS NULL OR la.code = sqlc.narg('account_code'))
  AND (sqlc.narg('from_time')::timestamptz IS NULL OR le.posted_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR le.posted_at <= sqlc.narg('to_time'))
  AND (
    sqlc.narg('cursor_time')::timestamptz IS NULL
    OR le.posted_at < sqlc.narg('cursor_time')
    OR (le.posted_at = sqlc.narg('cursor_time') AND le.id < sqlc.narg('cursor_id'))
  )
ORDER BY le.posted_at DESC, le.id DESC
LIMIT sqlc.arg('limit_val');

-- name: ListLedgerAccounts :many
SELECT *
FROM ledger_accounts
ORDER BY code ASC;
