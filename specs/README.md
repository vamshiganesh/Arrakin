# Arrakin — Engineering Specifications

This folder contains the implementation specifications for **Arrakin**, a reliability-first settlement, ledger, and payout engine.

| Document | Description |
|----------|-------------|
| [implementation-spec.md](./implementation-spec.md) | Primary technical spec: architecture, schema, processing design, APIs, testing, and phased delivery plan |

## Status

| Phase | Scope | Status |
|-------|-------|--------|
| 0 | Spec & repo plan | Complete |
| 1 | Foundation (schema, Docker, config) | Complete |
| 2 | Core domain & settlement calculator | Schema & seeds complete |
| 3 | Scheduler & job queue | Repository layer complete |
| 4 | Worker pool & payout processing | Scheduler & workers complete |
| 5 | Ledger & idempotency | Domain logic complete |
| 6 | Retry, dead-letter, reconciliation | Not started |
| 7 | HTTP API & OpenAPI | Not started |
| 8 | Admin UI shell | Not started |
| 9 | Integration tests & demo polish | Not started |

## Conventions

- Monetary amounts are stored as **integer minor units** (cents) in `BIGINT`.
- Timestamps are **UTC** (`TIMESTAMPTZ`).
- IDs are **UUID v7** (time-sortable) unless noted otherwise.
- API version prefix: `/api/v1`.
