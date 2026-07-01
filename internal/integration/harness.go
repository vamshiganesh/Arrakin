// Package integration contains end-to-end settlement flow tests against Docker Postgres.
// Run with: make test-integration (requires docker compose postgres on localhost:5432).
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/ledger"
	"github.com/vamshiganesh/arrakin/internal/reconciliation"
	"github.com/vamshiganesh/arrakin/internal/scheduler"
	"github.com/vamshiganesh/arrakin/internal/settlement/calculator"
	"github.com/vamshiganesh/arrakin/internal/settlement/orchestrator"
	"github.com/vamshiganesh/arrakin/internal/settlement/payout"
	"github.com/vamshiganesh/arrakin/internal/settlement/retry"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
	"github.com/vamshiganesh/arrakin/internal/worker"
)

// Fixture is an isolated investment + maturity pair for one test case.
type Fixture struct {
	InvestorID   uuid.UUID
	InvestmentID uuid.UUID
	MaturityID   uuid.UUID
}

// Stack wires the settlement engine components used in integration tests.
type Stack struct {
	Store      *store.Store
	Scheduler  *scheduler.Scheduler
	Orch       *orchestrator.Service
	Processor  *worker.Processor
	Recon      *reconciliation.Service
	Retry      retry.Policy
}

// RequireDB returns a live Postgres pool or skips the test.
func RequireDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://arrakin:arrakin@localhost:5432/arrakin?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

// NewStack builds scheduler, orchestrator, processor, and reconciliation services.
func NewStack(t *testing.T, pool *pgxpool.Pool) *Stack {
	t.Helper()
	st := store.New(pool)
	calc, err := calculator.New(calculator.Config{PlatformFeeBPS: 100, WithholdingTaxBPS: 1500})
	if err != nil {
		t.Fatal(err)
	}
	auditPub := audit.NewPublisher(st.Repos().Audit)
	orch := orchestrator.New(st, calc, auditPub, 5)
	retryPolicy := retry.Policy{BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	sched := scheduler.New(scheduler.Config{
		Interval:        time.Minute,
		ReaperInterval:  time.Minute,
		JobLeaseTimeout: 5 * time.Minute,
	}, orch, st, auditPub)
	processor := worker.NewProcessor(
		st,
		ledger.NewPostingService(st.Repos().Ledger),
		auditPub,
		payout.NewSimulator(),
		retryPolicy,
	)
	return &Stack{
		Store:     st,
		Scheduler: sched,
		Orch:      orch,
		Processor: processor,
		Recon:     reconciliation.New(st.Repos().Reconciliation),
		Retry:     retryPolicy,
	}
}

// SeedDueMaturity inserts an investor, investment, and past-due maturity schedule.
// simulationProfile may be "success", "transient_then_success", "terminal_failure", or "".
func SeedDueMaturity(t *testing.T, pool *pgxpool.Pool, ctx context.Context, simulationProfile string) Fixture {
	t.Helper()
	fix := Fixture{
		InvestorID:   uuid.New(),
		InvestmentID: uuid.New(),
		MaturityID:   uuid.New(),
	}
	suffix := fix.InvestmentID.String()[:8]

	if _, err := pool.Exec(ctx, `
		INSERT INTO investors (id, external_ref, display_name)
		VALUES ($1, $2, $3)
	`, fix.InvestorID, "integ-"+suffix, "Integration Investor "+suffix); err != nil {
		t.Fatalf("seed investor: %v", err)
	}

	var profile any
	if simulationProfile != "" {
		profile = simulationProfile
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO investments (
			id, investor_id, principal_cents, annual_rate_bps, term_days, currency, simulation_profile
		) VALUES ($1, $2, 1000000, 800, 365, 'USD', $3)
	`, fix.InvestmentID, fix.InvestorID, profile); err != nil {
		t.Fatalf("seed investment: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO maturity_schedules (id, investment_id, matures_at, status)
		VALUES ($1, $2, now() - interval '1 hour', 'pending')
	`, fix.MaturityID, fix.InvestmentID); err != nil {
		t.Fatalf("seed maturity: %v", err)
	}
	return fix
}

// EnqueueMaturity runs the orchestrator once and returns the job for the fixture maturity.
func EnqueueMaturity(t *testing.T, ctx context.Context, stack *Stack, fix Fixture) sqlc.SettlementJob {
	t.Helper()
	if _, err := stack.Scheduler.TickOnce(ctx); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}
	job, err := JobByMaturity(ctx, stack.Store.Pool(), fix.MaturityID)
	if err != nil {
		t.Fatalf("load job for maturity: %v", err)
	}
	return job
}

// ProcessJobOnce runs one worker cycle for the given worker ID.
func ProcessJobOnce(t *testing.T, ctx context.Context, stack *Stack, workerID string) {
	t.Helper()
	if err := stack.Processor.ProcessOne(ctx, workerID); err != nil && err != store.ErrNoJobAvailable {
		t.Fatalf("process one: %v", err)
	}
}

// ProcessUntilStatus polls the worker until the job reaches the target status or times out.
func ProcessUntilStatus(
	t *testing.T,
	ctx context.Context,
	stack *Stack,
	jobID uuid.UUID,
	target sqlc.SettlementJobStatus,
	workerID string,
	timeout time.Duration,
) sqlc.SettlementJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := JobByID(ctx, stack.Store.Pool(), jobID)
		if err != nil {
			t.Fatalf("load job: %v", err)
		}
		if job.Status == target {
			return job
		}
		if job.Status == sqlc.SettlementJobStatusFailed {
			FastForwardRetry(t, ctx, stack.Store.Pool(), jobID)
		}
		ProcessJobOnce(t, ctx, stack, workerID)
	}
	job, _ := JobByID(ctx, stack.Store.Pool(), jobID)
	t.Fatalf("timeout waiting for status %s, last status %s", target, job.Status)
	return job
}

// FastForwardRetry sets next_retry_at to the past so the job is immediately claimable.
func FastForwardRetry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, jobID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		UPDATE settlement_jobs
		SET next_retry_at = now() - interval '1 second'
		WHERE id = $1 AND status = 'failed'
	`, jobID); err != nil {
		t.Fatalf("fast-forward retry: %v", err)
	}
}

// JobByMaturity loads the settlement job for a maturity schedule.
func JobByMaturity(ctx context.Context, pool *pgxpool.Pool, maturityID uuid.UUID) (sqlc.SettlementJob, error) {
	return scanJob(pool.QueryRow(ctx, `
		SELECT id, maturity_schedule_id, investment_id, idempotency_key, status,
			principal_cents, gross_return_cents, platform_fee_cents, withholding_tax_cents, net_payout_cents,
			payout_reference, retry_count, max_retries, next_retry_at, processing_started_at, processing_owner,
			last_error, error_class, dead_letter_reason, created_at, updated_at, completed_at
		FROM settlement_jobs
		WHERE maturity_schedule_id = $1
	`, maturityID))
}

// JobByID loads a settlement job by primary key.
func JobByID(ctx context.Context, pool *pgxpool.Pool, jobID uuid.UUID) (sqlc.SettlementJob, error) {
	return scanJob(pool.QueryRow(ctx, `
		SELECT id, maturity_schedule_id, investment_id, idempotency_key, status,
			principal_cents, gross_return_cents, platform_fee_cents, withholding_tax_cents, net_payout_cents,
			payout_reference, retry_count, max_retries, next_retry_at, processing_started_at, processing_owner,
			last_error, error_class, dead_letter_reason, created_at, updated_at, completed_at
		FROM settlement_jobs
		WHERE id = $1
	`, jobID))
}

func scanJob(row pgx.Row) (sqlc.SettlementJob, error) {
	var j sqlc.SettlementJob
	err := row.Scan(
		&j.ID, &j.MaturityScheduleID, &j.InvestmentID, &j.IdempotencyKey, &j.Status,
		&j.PrincipalCents, &j.GrossReturnCents, &j.PlatformFeeCents, &j.WithholdingTaxCents, &j.NetPayoutCents,
		&j.PayoutReference, &j.RetryCount, &j.MaxRetries, &j.NextRetryAt, &j.ProcessingStartedAt, &j.ProcessingOwner,
		&j.LastError, &j.ErrorClass, &j.DeadLetterReason, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt,
	)
	return j, err
}

// LedgerLineCount returns ledger entry count for a settlement job.
func LedgerLineCount(ctx context.Context, pool *pgxpool.Pool, jobID uuid.UUID) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE settlement_job_id = $1`, jobID).Scan(&n)
	return n, err
}

// AttemptCount returns payout attempt count for a settlement job.
func AttemptCount(ctx context.Context, pool *pgxpool.Pool, jobID uuid.UUID) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM payout_attempts WHERE settlement_job_id = $1`, jobID).Scan(&n)
	return n, err
}

// JobPayoutTotals returns expected and succeeded net payout sums for specific job IDs.
func JobPayoutTotals(ctx context.Context, pool *pgxpool.Pool, jobIDs []uuid.UUID) (expected, succeeded int64, err error) {
	if len(jobIDs) == 0 {
		return 0, 0, fmt.Errorf("job ids required")
	}
	err = pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(net_payout_cents), 0)::bigint,
			COALESCE(SUM(net_payout_cents) FILTER (WHERE status = 'succeeded'), 0)::bigint
		FROM settlement_jobs
		WHERE id = ANY($1)
	`, jobIDs).Scan(&expected, &succeeded)
	return expected, succeeded, err
}

// CreatePendingJob inserts a pending settlement job directly (bypasses scheduler).
func CreatePendingJob(t *testing.T, ctx context.Context, stack *Stack, fix Fixture, netPayout int64) sqlc.SettlementJob {
	t.Helper()
	var job sqlc.SettlementJob
	err := stack.Store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		var err error
		job, _, err = stack.Store.Repos().SettlementJobs.CreateIdempotent(ctx, q, store.CreateJobParams{
			MaturityScheduleID:  store.UUIDToPgtype(fix.MaturityID),
			InvestmentID:        store.UUIDToPgtype(fix.InvestmentID),
			IdempotencyKey:      fmt.Sprintf("maturity:%s", fix.MaturityID),
			PrincipalCents:      1_000_000,
			GrossReturnCents:    80_000,
			PlatformFeeCents:    800,
			WithholdingTaxCents: 11_880,
			NetPayoutCents:      netPayout,
			MaxRetries:          5,
		})
		return err
	})
	if err != nil {
		t.Fatalf("create pending job: %v", err)
	}
	return job
}
