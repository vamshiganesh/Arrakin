DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS reconciliation_snapshots;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS ledger_accounts;
DROP TABLE IF EXISTS payout_attempts;
DROP TABLE IF EXISTS settlement_jobs;
DROP TABLE IF EXISTS maturity_schedules;
DROP TABLE IF EXISTS investments;
DROP TABLE IF EXISTS investors;

DROP TYPE IF EXISTS audit_actor_type;
DROP TYPE IF EXISTS error_class;
DROP TYPE IF EXISTS payout_attempt_status;
DROP TYPE IF EXISTS settlement_job_status;
DROP TYPE IF EXISTS maturity_status;
DROP TYPE IF EXISTS investment_status;

DROP EXTENSION IF EXISTS pgcrypto;
