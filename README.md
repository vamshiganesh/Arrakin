# Arrakin

Reliability-first **settlement, ledger, and payout engine** for a debt investment platform.

Arrakin identifies matured investments, computes settlement amounts, enqueues payout jobs, processes them concurrently with idempotency guarantees, writes immutable ledger entries, and exposes reconciliation and admin visibility.

## Documentation

- [Implementation specification](./specs/implementation-spec.md)
- [Specs index](./specs/README.md)

## Prerequisites

- Go 1.24+
- Docker and Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI (`migrate`)
- [sqlc](https://sqlc.dev/) CLI

## Local startup

### 1. Configure environment

```bash
cp .env.example .env
```

### 2. Start infrastructure

```bash
make docker-up
```

Wait until Postgres and Redis are healthy:

```bash
docker compose ps
```

### 3. Apply migrations

```bash
make migrate-up
```

### 4. Load demo seed data (optional)

```bash
make seed
```

### 5. Generate sqlc code (after schema or query changes)

```bash
make sqlc
```

### 6. Run the API

```bash
make run
```

The server listens on `http://localhost:8080` by default.

### 7. Verify health

```bash
curl -s http://localhost:8080/healthz | jq .
curl -s http://localhost:8080/readyz | jq .
```

Expected:

- `/healthz` → `{"status":"ok"}` (process liveness)
- `/readyz` → `{"status":"ok","checks":{"postgres":"ok","redis":"ok"}}` when dependencies are up

## Common commands

| Command | Description |
|---------|-------------|
| `make docker-up` | Start Postgres 16 and Redis 7 |
| `make docker-down` | Stop containers |
| `make migrate-up` | Apply migrations |
| `make migrate-down` | Roll back one migration |
| `make seed` | Load idempotent demo investors, investments, maturities |
| `make sqlc` | Regenerate typed SQL access code |
| `make build` | Build `bin/arrakin` |
| `make run` | Run API with hot `go run` |
| `make test` | Run tests |

## Project layout

```
cmd/arrakin/          Application entrypoint
internal/
  api/                HTTP routing and handlers
  config/             Environment configuration
  platform/           Shared infrastructure (db, redis, logging, httpx)
  store/sqlc/         sqlc-generated data access (generated)
migrations/           golang-migrate SQL migrations
sql/                  sqlc schema and queries
specs/                Engineering specifications
```

## License

See [LICENSE](./LICENSE).
