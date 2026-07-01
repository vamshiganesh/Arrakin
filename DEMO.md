# Arrakin Demo Walkthrough

Hands-on path through the settlement engine using the API and admin UI.

## Prerequisites

- Docker, Go 1.24+, `jq`, `curl`
- Optional: Node.js for the admin UI

## One-command demo

Self-contained (starts Docker, migrates, seeds, launches API, runs curls):

```bash
make demo
```

With the API already running (`make run` in another terminal):

```bash
BOOTSTRAP=0 ./scripts/demo.sh
```

## Manual walkthrough

### 1. Start stack

```bash
cp .env.example .env
make docker-up && make migrate-up && make seed
make run
```

### 2. Trigger settlement

```bash
curl -s -X POST http://localhost:8080/api/v1/admin/scheduler/tick \
  -H 'X-API-Key: dev-local-api-key' \
  -H 'Idempotency-Key: walkthrough-tick-1' | jq .
```

The seed cohort includes:

| Investment | Profile | Expected outcome |
|------------|---------|------------------|
| INV-DEMO-001 | success | Succeeds on first attempt |
| INV-DEMO-002 | transient_then_success | Retries, then succeeds |
| INV-DEMO-003 | terminal_failure | Ends in dead_letter |
| INV-DEMO-004 | success | Additional succeeded volume |
| INV-DEMO-005 | (none) | Production-style success |

### 3. Inspect jobs

```bash
curl -s 'http://localhost:8080/api/v1/settlement-jobs?status=dead_letter&limit=5' | jq .
curl -s 'http://localhost:8080/api/v1/settlement-jobs?status=succeeded&limit=5' | jq .
```

### 4. Job detail and attempts

```bash
JOB_ID=$(curl -s 'http://localhost:8080/api/v1/settlement-jobs?status=dead_letter&limit=1' | jq -r '.items[0].id')
curl -s "http://localhost:8080/api/v1/settlement-jobs/${JOB_ID}" | jq .
curl -s "http://localhost:8080/api/v1/settlement-jobs/${JOB_ID}/attempts" | jq .
```

### 5. Replay dead-letter job

```bash
curl -s -X POST "http://localhost:8080/api/v1/settlement-jobs/${JOB_ID}/replay" \
  -H 'X-API-Key: dev-local-api-key' \
  -H 'Idempotency-Key: walkthrough-replay-1' | jq .
```

Terminal-failure profile returns to dead_letter after replay (by design).

### 6. Reconciliation

```bash
curl -s -X POST http://localhost:8080/api/v1/reconciliation/run \
  -H 'X-API-Key: dev-local-api-key' \
  -H 'Idempotency-Key: walkthrough-recon-1' | jq .
```

Re-run with the same idempotency key; response is replayed (`X-Idempotency-Replayed: true`).

### 7. Ledger and audit

```bash
curl -s "http://localhost:8080/api/v1/ledger/entries?settlement_job_id=${JOB_ID}&limit=10" | jq .
curl -s 'http://localhost:8080/api/v1/audit/events?limit=10' | jq .
```

### 8. Admin UI

```bash
cd web/admin && cp .env.example .env && npm install && npm run dev
```

Open http://localhost:5173 — overview, jobs, ledger, reconciliation, audit.

## Idempotency proof

1. Run scheduler tick twice for the same matured investments → second tick creates **0** new jobs.
2. POST reconciliation with the same `Idempotency-Key` → identical JSON, replay header set.
3. Succeeded jobs keep a fixed ledger line count after worker re-polls (integration test).

## API collection

Bruno requests live under `api/bruno/`. Import the folder in [Bruno](https://www.usebruno.com/) and select the `local` environment.
