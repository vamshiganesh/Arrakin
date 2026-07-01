#!/usr/bin/env bash
# Arrakin end-to-end demo: seed cohort → scheduler tick → reconciliation → idempotent replay.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

API_BASE="${API_BASE:-http://localhost:8080}"
API_KEY="${API_KEY:-dev-local-api-key}"
DATABASE_URL="${DATABASE_URL:-postgres://arrakin:arrakin@localhost:5432/arrakin?sslmode=disable}"
REDIS_URL="${REDIS_URL:-redis://localhost:6379/0}"
BOOTSTRAP="${BOOTSTRAP:-0}"
SERVER_PID=""

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: $1 is required" >&2
    exit 1
  fi
}

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

wait_for_api() {
  local i
  for i in $(seq 1 45); do
    if curl -sf "$API_BASE/healthz" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "error: API not reachable at $API_BASE" >&2
  return 1
}

wait_for_postgres() {
  local i
  for i in $(seq 1 30); do
    if psql "$DATABASE_URL" -c 'SELECT 1' >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "error: postgres not reachable" >&2
  return 1
}

bootstrap_stack() {
  echo "==> Starting infrastructure"
  make docker-up
  wait_for_postgres
  make migrate-up seed

  echo "==> Starting API server"
  export APP_ENV=development
  export DATABASE_URL REDIS_URL API_KEY
  go run ./cmd/arrakin &
  SERVER_PID=$!
  wait_for_api
}

api_ready() {
  curl -sf "$API_BASE/healthz" >/dev/null 2>&1
}

step() {
  echo
  echo "==> $1"
}

require_cmd curl
require_cmd jq

if [[ "${1:-}" == "--bootstrap" ]]; then
  BOOTSTRAP=1
fi

if ! api_ready; then
  if [[ "$BOOTSTRAP" == "1" ]]; then
    bootstrap_stack
  else
    echo "API is not running at $API_BASE"
    echo "Start it with: make run"
    echo "Or run a self-contained demo: make demo"
    exit 1
  fi
fi

step "Health check"
curl -s "$API_BASE/healthz" | jq .

step "Scheduler tick (enqueue due maturities)"
TICK_1=$(curl -s -X POST "$API_BASE/api/v1/admin/scheduler/tick" \
  -H "X-API-Key: $API_KEY" \
  -H "Idempotency-Key: demo-tick-$(date +%s)")
echo "$TICK_1" | jq .
CREATED=$(echo "$TICK_1" | jq -r '.jobs_created // 0')
echo "Jobs created: $CREATED"

step "List jobs by status"
for status in pending processing succeeded failed dead_letter; do
  COUNT=$(curl -s "$API_BASE/api/v1/settlement-jobs?status=$status&limit=50" | jq -r '.items | length')
  printf "  %-14s %s\n" "$status" "$COUNT"
done

step "Reconciliation snapshot"
RECON_KEY="demo-recon-$(date +%s)"
RECON_1=$(curl -s -X POST "$API_BASE/api/v1/reconciliation/run" \
  -H "X-API-Key: $API_KEY" \
  -H "Idempotency-Key: $RECON_KEY")
echo "$RECON_1" | jq '{discrepancy_cents: .summary.discrepancy_cents, by_status: .summary.by_status}'

step "Idempotent reconciliation replay (same Idempotency-Key)"
RECON_2=$(curl -si -X POST "$API_BASE/api/v1/reconciliation/run" \
  -H "X-API-Key: $API_KEY" \
  -H "Idempotency-Key: $RECON_KEY")
REPLAYED=$(echo "$RECON_2" | awk 'BEGIN{IGNORECASE=1} /^X-Idempotency-Replayed:/ {print $2}' | tr -d '\r')
BODY=$(echo "$RECON_2" | awk 'BEGIN{body=0} /^\r?$/ {body=1; next} body {print}')
echo "X-Idempotency-Replayed: ${REPLAYED:-false}"
echo "$BODY" | jq '{discrepancy_cents: .summary.discrepancy_cents}'

step "Repeat scheduler tick (idempotent enqueue)"
TICK_2=$(curl -s -X POST "$API_BASE/api/v1/admin/scheduler/tick" \
  -H "X-API-Key: $API_KEY" \
  -H "Idempotency-Key: demo-tick-repeat-$(date +%s)")
echo "$TICK_2" | jq .
echo "Second tick should create 0 new jobs for already-settled maturities."

step "Audit tail"
curl -s "$API_BASE/api/v1/audit/events?limit=5" | jq '.items[] | {action, occurred_at}'

echo
echo "Demo complete."
echo "Open the admin UI: cd web/admin && npm run dev  →  http://localhost:5173"
