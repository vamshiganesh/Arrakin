package worker_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/ledger"
	"github.com/vamshiganesh/arrakin/internal/scheduler"
	"github.com/vamshiganesh/arrakin/internal/settlement/calculator"
	"github.com/vamshiganesh/arrakin/internal/settlement/orchestrator"
	"github.com/vamshiganesh/arrakin/internal/settlement/payout"
	"github.com/vamshiganesh/arrakin/internal/settlement/retry"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
	"github.com/vamshiganesh/arrakin/internal/worker"
)

func testPool(t *testing.T) (*pgxpool.Pool, context.Context) {
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

func newTestStack(t *testing.T, pool *pgxpool.Pool) (*store.Store, *scheduler.Scheduler, *worker.Processor) {
	t.Helper()
	st := store.New(pool)
	calc, err := calculator.New(calculator.Config{PlatformFeeBPS: 100, WithholdingTaxBPS: 1500})
	if err != nil {
		t.Fatal(err)
	}
	auditPub := audit.NewPublisher(st.Repos().Audit)
	orch := orchestrator.New(st, calc, auditPub, 5)
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
		retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Second},
	)
	return st, sched, processor
}

func TestOrchestratorEnqueueIdempotent(t *testing.T) {
	pool, ctx := testPool(t)
	_, sched, _ := newTestStack(t, pool)

	first, err := sched.TickOnce(ctx)
	if err != nil {
		t.Fatalf("first tick: %v", err)
	}
	second, err := sched.TickOnce(ctx)
	if err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected no new jobs on second tick, got %d (first=%d)", second, first)
	}
}

func TestProcessorRetryThenSuccess(t *testing.T) {
	pool, ctx := testPool(t)
	_, sched, processor := newTestStack(t, pool)

	if _, err := sched.TickOnce(ctx); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	investmentID := uuid.MustParse("b2000001-0002-4002-8002-000000000002")
	workerID := "test-retry-worker"

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_ = processor.ProcessOne(ctx, workerID)

		job, err := getJobByInvestment(ctx, pool, investmentID)
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			t.Fatalf("load job: %v", err)
		}
		if job.Status == sqlc.SettlementJobStatusSucceeded {
			return
		}
		if job.Status == sqlc.SettlementJobStatusFailed {
			if _, err := pool.Exec(ctx, `
				UPDATE settlement_jobs SET next_retry_at = now() - interval '1 second'
				WHERE id = $1
			`, job.ID); err != nil {
				t.Fatalf("fast-forward retry: %v", err)
			}
		}
	}
	t.Fatal("expected transient_then_success job to eventually succeed")
}

func TestConcurrentWorkersNoDuplicateSuccess(t *testing.T) {
	pool, ctx := testPool(t)
	_, sched, processor := newTestStack(t, pool)
	if _, err := sched.TickOnce(ctx); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "concurrent-worker"
			for j := 0; j < 20; j++ {
				_ = processor.ProcessOne(ctx, id)
			}
		}(i)
	}
	wg.Wait()

	var succeeded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM settlement_jobs WHERE status = 'succeeded'`).Scan(&succeeded); err != nil {
		t.Fatalf("count succeeded: %v", err)
	}
	if succeeded == 0 {
		t.Fatal("expected at least one succeeded job")
	}

	var duplicateRefs int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT payout_reference FROM settlement_jobs
			WHERE payout_reference IS NOT NULL
			GROUP BY payout_reference HAVING count(*) > 1
		) d
	`).Scan(&duplicateRefs); err != nil {
		t.Fatalf("duplicate refs: %v", err)
	}
	if duplicateRefs > 0 {
		t.Fatal("found duplicate payout references")
	}
}

func getJobByInvestment(ctx context.Context, pool *pgxpool.Pool, investmentID uuid.UUID) (sqlc.SettlementJob, error) {
	row := pool.QueryRow(ctx, `
		SELECT id, maturity_schedule_id, investment_id, idempotency_key, status,
			principal_cents, gross_return_cents, platform_fee_cents, withholding_tax_cents, net_payout_cents,
			payout_reference, retry_count, max_retries, next_retry_at, processing_started_at, processing_owner,
			last_error, error_class, dead_letter_reason, created_at, updated_at, completed_at
		FROM settlement_jobs
		WHERE investment_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, investmentID)
	var j sqlc.SettlementJob
	err := row.Scan(
		&j.ID, &j.MaturityScheduleID, &j.InvestmentID, &j.IdempotencyKey, &j.Status,
		&j.PrincipalCents, &j.GrossReturnCents, &j.PlatformFeeCents, &j.WithholdingTaxCents, &j.NetPayoutCents,
		&j.PayoutReference, &j.RetryCount, &j.MaxRetries, &j.NextRetryAt, &j.ProcessingStartedAt, &j.ProcessingOwner,
		&j.LastError, &j.ErrorClass, &j.DeadLetterReason, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt,
	)
	return j, err
}
