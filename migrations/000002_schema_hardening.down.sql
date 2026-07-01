DROP INDEX IF EXISTS idx_audit_events_correlation_id;
DROP INDEX IF EXISTS idx_audit_events_action;
DROP INDEX IF EXISTS idx_idempotency_keys_scope_created;
DROP INDEX IF EXISTS ux_payout_attempts_payout_reference;
DROP INDEX IF EXISTS idx_ledger_entries_account_posted;
DROP INDEX IF EXISTS idx_reconciliation_snapshots_discrepancy;
DROP INDEX IF EXISTS idx_settlement_jobs_succeeded_completed;
DROP INDEX IF EXISTS idx_settlement_jobs_status_created;
DROP INDEX IF EXISTS idx_settlement_jobs_processing_lease;
DROP INDEX IF EXISTS idx_settlement_jobs_claimable;
DROP INDEX IF EXISTS idx_maturity_schedules_status_matures_at;
DROP INDEX IF EXISTS ux_ledger_entries_group_account_side;

DROP TRIGGER IF EXISTS trg_ledger_entries_immutable ON ledger_entries;
DROP FUNCTION IF EXISTS prevent_ledger_entry_mutation();

DROP TRIGGER IF EXISTS trg_settlement_jobs_updated_at ON settlement_jobs;
DROP FUNCTION IF EXISTS set_updated_at();

ALTER TABLE settlement_jobs
    DROP CONSTRAINT IF EXISTS chk_settlement_jobs_succeeded_payout_ref,
    DROP CONSTRAINT IF EXISTS chk_settlement_jobs_dead_letter_reason,
    DROP CONSTRAINT IF EXISTS chk_settlement_jobs_amounts_nonneg,
    DROP CONSTRAINT IF EXISTS chk_settlement_jobs_max_retries_nonneg,
    DROP CONSTRAINT IF EXISTS chk_settlement_jobs_retry_count_nonneg;
