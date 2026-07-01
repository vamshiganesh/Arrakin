# Arrakin Admin UI

Operations console for settlement jobs, ledger, reconciliation, and audit.

## Prerequisites

1. Backend API running on port **8080** (`make run` from repo root)
2. Docker Postgres seeded (`make docker-up && make migrate-up && make seed`)
3. Node.js 18+

## Setup

```bash
cd web/admin
cp .env.example .env
npm install
```

Set `VITE_API_KEY` in `.env` to match backend `API_KEY` (default: `dev-local-api-key`).

## Development

```bash
npm run dev
```

Open [http://localhost:5173](http://localhost:5173).

Vite proxies `/api` to `http://localhost:8080`, so no CORS configuration is required locally.

## Production build

```bash
npm run build
npm run preview
```

## Pages

| Route | Purpose |
|-------|---------|
| `/` | Overview, reconciliation summary, scheduler trigger |
| `/jobs` | Settlement job list with status filter |
| `/jobs/:id` | Job detail, payout attempts, replay/requeue |
| `/ledger` | Ledger entries with job and account filters |
| `/reconciliation` | Latest snapshot, run snapshot, history |
| `/audit` | Audit event timeline |

## Full stack (from repo root)

```bash
make docker-up && make migrate-up && make seed
make run                    # terminal 1: API on :8080
make admin-dev              # terminal 2: UI on :5173
```
