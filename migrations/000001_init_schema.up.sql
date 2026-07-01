-- Arrakin initial schema: domain tables for settlement, ledger, and audit.
-- See specs/implementation-spec.md §6 for design rationale.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE investment_status AS ENUM (
    'active',
    'matured',
    'settled',
    'cancelled'
);

CREATE TYPE maturity_status AS ENUM (
    'pending',
    'processing',
    'settled',
    'skipped'
);

CREATE TYPE settlement_job_status AS ENUM (
    'pending',
    'processing',
    'succeeded',
    'failed',
    'dead_letter'
);

CREATE TYPE payout_attempt_status AS ENUM (
    'started',
    'succeeded',
    'failed'
);

CREATE TYPE error_class AS ENUM (
    'transient',
    'terminal'
);

CREATE TYPE audit_actor_type AS ENUM (
    'system',
    'admin',
    'api'
);

CREATE TABLE investors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_ref TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE investments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    investor_id UUID NOT NULL REFERENCES investors (id),
    principal_cents BIGINT NOT NULL CHECK (principal_cents > 0),
    annual_rate_bps INT NOT NULL CHECK (annual_rate_bps >= 0),
    term_days INT NOT NULL CHECK (term_days > 0),
    status investment_status NOT NULL DEFAULT 'active',
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    simulation_profile TEXT CHECK (
        simulation_profile IS NULL
        OR simulation_profile IN ('success', 'transient_then_success', 'terminal_failure')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_investments_investor_id ON investments (investor_id);
CREATE INDEX idx_investments_status ON investments (status);

CREATE TABLE maturity_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    investment_id UUID NOT NULL UNIQUE REFERENCES investments (id),
    matures_at TIMESTAMPTZ NOT NULL,
    status maturity_status NOT NULL DEFAULT 'pending',
    settled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_maturity_schedules_pending_due
    ON maturity_schedules (matures_at)
    WHERE status = 'pending';

CREATE TABLE settlement_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    maturity_schedule_id UUID NOT NULL UNIQUE REFERENCES maturity_schedules (id),
    investment_id UUID NOT NULL REFERENCES investments (id),
    idempotency_key TEXT NOT NULL UNIQUE,
    status settlement_job_status NOT NULL DEFAULT 'pending',
    principal_cents BIGINT NOT NULL DEFAULT 0,
    gross_return_cents BIGINT NOT NULL DEFAULT 0,
    platform_fee_cents BIGINT NOT NULL DEFAULT 0,
    withholding_tax_cents BIGINT NOT NULL DEFAULT 0,
    net_payout_cents BIGINT NOT NULL DEFAULT 0,
    payout_reference TEXT UNIQUE,
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMPTZ,
    processing_started_at TIMESTAMPTZ,
    processing_owner TEXT,
    last_error TEXT,
    error_class error_class,
    dead_letter_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_settlement_jobs_status_next_retry
    ON settlement_jobs (status, next_retry_at);

CREATE INDEX idx_settlement_jobs_status_processing_started
    ON settlement_jobs (status, processing_started_at);

CREATE INDEX idx_settlement_jobs_investment_id ON settlement_jobs (investment_id);

CREATE TABLE payout_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_job_id UUID NOT NULL REFERENCES settlement_jobs (id),
    attempt_number INT NOT NULL CHECK (attempt_number > 0),
    status payout_attempt_status NOT NULL,
    payout_reference TEXT,
    error_message TEXT,
    error_class error_class,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    UNIQUE (settlement_job_id, attempt_number)
);

CREATE INDEX idx_payout_attempts_settlement_job_id ON payout_attempts (settlement_job_id);

CREATE TABLE ledger_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    account_type TEXT NOT NULL CHECK (account_type IN ('liability', 'asset', 'revenue')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_group_id UUID NOT NULL,
    settlement_job_id UUID NOT NULL REFERENCES settlement_jobs (id),
    account_id UUID NOT NULL REFERENCES ledger_accounts (id),
    side CHAR(1) NOT NULL CHECK (side IN ('D', 'C')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    description TEXT NOT NULL DEFAULT '',
    posted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_ledger_entries_settlement_job_id ON ledger_entries (settlement_job_id);
CREATE INDEX idx_ledger_entries_posted_at ON ledger_entries (posted_at);
CREATE INDEX idx_ledger_entries_entry_group_id ON ledger_entries (entry_group_id);

CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL,
    scope TEXT NOT NULL,
    request_hash TEXT,
    response_status INT,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (scope, key)
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);

CREATE TABLE reconciliation_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expected_job_count INT NOT NULL DEFAULT 0,
    expected_total_cents BIGINT NOT NULL DEFAULT 0,
    succeeded_count INT NOT NULL DEFAULT 0,
    succeeded_total_cents BIGINT NOT NULL DEFAULT 0,
    pending_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    dead_letter_count INT NOT NULL DEFAULT 0,
    discrepancy_cents BIGINT NOT NULL DEFAULT 0,
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_reconciliation_snapshots_snapshot_at ON reconciliation_snapshots (snapshot_at DESC);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_type audit_actor_type NOT NULL,
    actor_id TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    correlation_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_audit_events_entity ON audit_events (entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_events_occurred_at ON audit_events (occurred_at DESC);
