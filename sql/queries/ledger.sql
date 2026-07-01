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
