# Production Concerns

Operational notes for running Arrakin beyond local development.

## Concurrency and locking

- Workers claim jobs with `SELECT … FOR UPDATE SKIP LOCKED` inside a transaction.
- Each processing attempt commits status, payout attempt, and ledger lines in **one database transaction**.
- Stale `processing` jobs are reclaimed when `processing_started_at` exceeds `JOB_LEASE_TIMEOUT` (default 5m).
- Scheduler leader election uses Redis when available; falls back to Postgres advisory locks.

## Indexes and query patterns

- Settlement jobs are filtered by `status`, `investment_id`, and `created_at` for admin list APIs.
- Ledger entries are indexed by `settlement_job_id` and `account_code`.
- Maturity enqueue relies on a unique constraint: one active job per maturity schedule.
- `payout_reference` is unique per completed payout to block duplicate completion.

## Idempotency

| Layer | Mechanism |
|-------|-----------|
| HTTP POST | `Idempotency-Key` header + `idempotency_keys` table |
| Job creation | `idempotency_key` per maturity (`maturity:{id}`) |
| Payout completion | Unique `payout_reference` from gateway |

Duplicate HTTP requests with the same key return the stored response with `X-Idempotency-Replayed: true`.

## Retries and dead letter

- Transient payout failures increment `retry_count` and set `next_retry_at` via exponential backoff.
- After `MAX_RETRIES` (default 5), jobs move to `dead_letter`.
- Dead-letter jobs require explicit admin **replay**; failed (retryable) jobs use **requeue**.

## Auditing

- Job status transitions and admin actions append rows to `audit_events`.
- Structured logs (`log/slog` JSON) carry `request_id`, `job_id`, and `investment_id` for correlation.
- Audit events are queryable via `GET /api/v1/audit/events`.

## Security

- Mutating admin routes require `X-API-Key` when `APP_ENV` is not `development`.
- Place the admin UI behind VPN or SSO in production; the UI sends the API key from environment config.
- Do not commit `.env` files or production keys.

## Money handling

- All amounts are `BIGINT` USD cents; no floating point in settlement paths.
- Ledger is append-only; corrections are new entries, not updates.

## Deferred (post-v1)

- Prometheus `/metrics` HTTP scrape endpoint (counters exist in-process).
- Distributed tracing (request/job correlation via logs today).
- Horizontal worker split via separate process roles.
- Real payout gateway adapters (ACH, wire).
