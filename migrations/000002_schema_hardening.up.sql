-- Production hardening: immutability guards, worker/scheduler indexes, and domain comments.
-- Complements 000001_init_schema with operational constraints for settlement reliability.

-- ---------------------------------------------------------------------------
-- Column and table documentation
-- ---------------------------------------------------------------------------

COMMENT ON TABLE investments IS
    'Debt positions held on the platform; principal and rate terms drive settlement calculation.';

COMMENT ON COLUMN investments.simulation_profile IS
    'Demo-only payout behavior: success | transient_then_success | terminal_failure. NULL for production investments.';

COMMENT ON TABLE maturity_schedules IS
    'When an investment becomes due for settlement. Scheduler scans pending rows where matures_at <= now().';

COMMENT ON TABLE settlement_jobs IS
    'Unit of settlement work. One job per maturity (enforced by UNIQUE on maturity_schedule_id).';

COMMENT ON COLUMN settlement_jobs.idempotency_key IS
    'Internal enqueue dedupe key, typically derived from maturity_schedule_id.';

COMMENT ON COLUMN settlement_jobs.payout_reference IS
    'External payout rail reference; UNIQUE when set to prevent duplicate completion.';

COMMENT ON COLUMN settlement_jobs.retry_count IS
    'Number of failed attempts; compared against max_retries before dead-letter.';

COMMENT ON COLUMN settlement_jobs.next_retry_at IS
    'Workers skip failed jobs until this timestamp (exponential backoff).';

COMMENT ON COLUMN settlement_jobs.processing_started_at IS
    'Lease start for in-flight work; stale leases are reclaimed by the reaper.';

COMMENT ON COLUMN settlement_jobs.dead_letter_reason IS
    'Human-readable terminal failure reason when status = dead_letter.';

COMMENT ON TABLE payout_attempts IS
    'Append-only execution history for each settlement job attempt.';

COMMENT ON TABLE ledger_entries IS
    'Immutable double-entry lines. Corrections require compensating entries, never UPDATE/DELETE.';

COMMENT ON COLUMN ledger_entries.entry_group_id IS
    'Groups debit/credit lines that form one balanced posting for a settlement event.';

COMMENT ON TABLE idempotency_keys IS
    'HTTP and admin mutation dedupe store; replay returns cached response within TTL.';

COMMENT ON TABLE reconciliation_snapshots IS
    'Point-in-time comparison of expected vs processed settlement totals.';

COMMENT ON TABLE audit_events IS
    'Append-only trail of state transitions and administrative actions.';

-- ---------------------------------------------------------------------------
-- settlement_jobs: lifecycle integrity
-- ---------------------------------------------------------------------------

ALTER TABLE settlement_jobs
    ADD CONSTRAINT chk_settlement_jobs_retry_count_nonneg
        CHECK (retry_count >= 0),
    ADD CONSTRAINT chk_settlement_jobs_max_retries_nonneg
        CHECK (max_retries >= 0),
    ADD CONSTRAINT chk_settlement_jobs_amounts_nonneg
        CHECK (
            principal_cents >= 0
            AND gross_return_cents >= 0
            AND platform_fee_cents >= 0
            AND withholding_tax_cents >= 0
            AND net_payout_cents >= 0
        ),
    ADD CONSTRAINT chk_settlement_jobs_dead_letter_reason
        CHECK (
            status <> 'dead_letter'
            OR dead_letter_reason IS NOT NULL
        ),
    ADD CONSTRAINT chk_settlement_jobs_succeeded_payout_ref
        CHECK (
            status <> 'succeeded'
            OR payout_reference IS NOT NULL
        );

-- ---------------------------------------------------------------------------
-- Automatic updated_at on settlement_jobs
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_settlement_jobs_updated_at
    BEFORE UPDATE ON settlement_jobs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- ledger_entries: immutability (append-only financial record)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION prevent_ledger_entry_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries are immutable; post a compensating entry instead'
        USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ledger_entries_immutable
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_ledger_entry_mutation();

-- Prevent duplicate account line within the same posting group.
CREATE UNIQUE INDEX ux_ledger_entries_group_account_side
    ON ledger_entries (entry_group_id, account_id, side);

-- ---------------------------------------------------------------------------
-- Scheduler scan index (pending maturities due for settlement)
-- ---------------------------------------------------------------------------

CREATE INDEX idx_maturity_schedules_status_matures_at
    ON maturity_schedules (status, matures_at);

-- ---------------------------------------------------------------------------
-- Worker claim index (pending/failed jobs eligible for pickup)
-- ---------------------------------------------------------------------------

CREATE INDEX idx_settlement_jobs_claimable
    ON settlement_jobs (next_retry_at ASC NULLS FIRST, created_at ASC)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_settlement_jobs_processing_lease
    ON settlement_jobs (processing_started_at)
    WHERE status = 'processing';

CREATE INDEX idx_settlement_jobs_status_created
    ON settlement_jobs (status, created_at DESC);

-- ---------------------------------------------------------------------------
-- Reconciliation and reporting indexes
-- ---------------------------------------------------------------------------

CREATE INDEX idx_settlement_jobs_succeeded_completed
    ON settlement_jobs (completed_at DESC)
    WHERE status = 'succeeded';

CREATE INDEX idx_reconciliation_snapshots_discrepancy
    ON reconciliation_snapshots (snapshot_at DESC)
    WHERE discrepancy_cents <> 0;

CREATE INDEX idx_ledger_entries_account_posted
    ON ledger_entries (account_id, posted_at DESC);

-- ---------------------------------------------------------------------------
-- Payout attempt and idempotency indexes
-- ---------------------------------------------------------------------------

CREATE UNIQUE INDEX ux_payout_attempts_payout_reference
    ON payout_attempts (payout_reference)
    WHERE payout_reference IS NOT NULL;

CREATE INDEX idx_idempotency_keys_scope_created
    ON idempotency_keys (scope, created_at DESC);

-- ---------------------------------------------------------------------------
-- Audit lookup indexes
-- ---------------------------------------------------------------------------

CREATE INDEX idx_audit_events_action
    ON audit_events (action, occurred_at DESC);

CREATE INDEX idx_audit_events_correlation_id
    ON audit_events (correlation_id)
    WHERE correlation_id <> '';
