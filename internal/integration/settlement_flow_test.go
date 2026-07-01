package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

func TestMaturedInvestmentCreatesSettlementJob(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)
	fix := SeedDueMaturity(t, pool, ctx, "success")

	if _, err := stack.Scheduler.TickOnce(ctx); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	job, err := JobByMaturity(ctx, pool, fix.MaturityID)
	if err != nil {
		t.Fatalf("load job for maturity: %v", err)
	}
	if job.Status != sqlc.SettlementJobStatusPending {
		t.Fatalf("expected pending job, got %s", job.Status)
	}
	if job.NetPayoutCents <= 0 {
		t.Fatal("expected positive net payout on created job")
	}

	jobID, err := store.PgtypeToUUID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := AttemptCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("expected no payout attempts before processing, got %d", attempts)
	}
}

func TestSuccessfulPayoutPostsLedgerExactlyOnce(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)
	fix := SeedDueMaturity(t, pool, ctx, "success")
	job := CreateJobForFixture(t, ctx, stack, fix)

	jobID, err := store.PgtypeToUUID(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	final := ProcessJobUntilStatus(t, ctx, stack, jobID, sqlc.SettlementJobStatusSucceeded, "ledger-once-worker", 5*time.Second)
	if final.PayoutReference == nil || *final.PayoutReference == "" {
		t.Fatal("expected payout reference on succeeded job")
	}

	lines, err := LedgerLineCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 4 {
		t.Fatalf("expected 4 ledger lines, got %d", lines)
	}

	// Re-processing a succeeded job must not be claimable; ledger count stays 4.
	before := lines
	for i := 0; i < 3; i++ {
		ProcessJobOnce(t, ctx, stack, "ledger-once-worker")
	}
	lines, err = LedgerLineCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if lines != before {
		t.Fatalf("ledger lines changed on re-process: before=%d after=%d", before, lines)
	}
}

func TestRetryableFailureIncrementsAttemptsAndSchedulesRetry(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)
	fix := SeedDueMaturity(t, pool, ctx, "transient_then_success")
	job := CreateJobForFixture(t, ctx, stack, fix)

	jobID, err := store.PgtypeToUUID(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	ProcessJobOnce(t, ctx, stack, "retry-test-worker")

	afterFail, err := JobByID(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if afterFail.Status != sqlc.SettlementJobStatusFailed {
		t.Fatalf("expected failed after transient attempt, got %s", afterFail.Status)
	}
	if afterFail.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", afterFail.RetryCount)
	}
	if !afterFail.NextRetryAt.Valid {
		t.Fatal("expected next_retry_at to be set")
	}
	if afterFail.NextRetryAt.Time.Before(time.Now()) {
		t.Fatal("expected next_retry_at in the future")
	}

	attempts, err := AttemptCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 payout attempt, got %d", attempts)
	}
}

func TestTerminalFailureGoesToDeadLetter(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)
	fix := SeedDueMaturity(t, pool, ctx, "terminal_failure")
	job := CreateJobForFixture(t, ctx, stack, fix)

	jobID, err := store.PgtypeToUUID(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	ProcessJobOnce(t, ctx, stack, "terminal-worker")

	final, err := JobByID(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != sqlc.SettlementJobStatusDeadLetter {
		t.Fatalf("expected dead_letter, got %s", final.Status)
	}
	if final.DeadLetterReason == nil || *final.DeadLetterReason == "" {
		t.Fatal("expected dead_letter_reason")
	}

	lines, err := LedgerLineCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 0 {
		t.Fatalf("terminal failure must not post ledger, got %d lines", lines)
	}
}

func TestRerunTriggerDoesNotDuplicateSettlement(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)
	fix := SeedDueMaturity(t, pool, ctx, "success")

	first, err := stack.Scheduler.TickOnce(ctx)
	if err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if first == 0 {
		t.Fatal("expected first tick to create a job")
	}

	jobAfterFirst, err := JobByMaturity(ctx, pool, fix.MaturityID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}

	second, err := stack.Scheduler.TickOnce(ctx)
	if err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected second tick to create 0 jobs, got %d", second)
	}

	jobAfterSecond, err := JobByMaturity(ctx, pool, fix.MaturityID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if jobAfterFirst.ID != jobAfterSecond.ID {
		t.Fatal("re-tick must not create a duplicate settlement job")
	}

	var jobCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM settlement_jobs WHERE maturity_schedule_id = $1
	`, fix.MaturityID).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if jobCount != 1 {
		t.Fatalf("expected exactly one job for maturity, got %d", jobCount)
	}
}

func TestReconciliationReflectsProcessedVsExpectedTotals(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)

	successFix := SeedDueMaturity(t, pool, ctx, "success")
	deadFix := SeedDueMaturity(t, pool, ctx, "terminal_failure")

	successJob := EnqueueMaturity(t, ctx, stack, successFix)
	deadJob := EnqueueMaturity(t, ctx, stack, deadFix)

	successID, _ := store.PgtypeToUUID(successJob.ID)
	deadID, _ := store.PgtypeToUUID(deadJob.ID)

	ProcessUntilStatus(t, ctx, stack, successID, sqlc.SettlementJobStatusSucceeded, "recon-success", 5*time.Second)
	ProcessUntilStatus(t, ctx, stack, deadID, sqlc.SettlementJobStatusDeadLetter, "recon-dead", 5*time.Second)

	expected, succeeded, err := JobPayoutTotals(ctx, pool, []uuid.UUID{successID, deadID})
	if err != nil {
		t.Fatal(err)
	}
	if succeeded != successJob.NetPayoutCents {
		t.Fatalf("succeeded total mismatch: got %d want %d", succeeded, successJob.NetPayoutCents)
	}
	discrepancy := expected - succeeded
	if discrepancy != deadJob.NetPayoutCents {
		t.Fatalf("fixture discrepancy mismatch: got %d want %d", discrepancy, deadJob.NetPayoutCents)
	}

	var snapshot sqlc.ReconciliationSnapshot
	err = stack.Store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		var err error
		snapshot, err = stack.Recon.RunSnapshot(ctx, q)
		return err
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if snapshot.DiscrepancyCents != snapshot.ExpectedTotalCents-snapshot.SucceededTotalCents {
		t.Fatalf("snapshot internal math wrong: discrepancy=%d expected-succeeded=%d",
			snapshot.DiscrepancyCents, snapshot.ExpectedTotalCents-snapshot.SucceededTotalCents)
	}
	if snapshot.DiscrepancyCents < discrepancy {
		t.Fatalf("global discrepancy %d should be at least fixture gap %d", snapshot.DiscrepancyCents, discrepancy)
	}
}

func TestConcurrentWorkersDoNotDoubleCompleteJob(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)

	const jobs = 5
	jobIDs := make([]uuid.UUID, 0, jobs)
	for i := 0; i < jobs; i++ {
		fix := SeedDueMaturity(t, pool, ctx, "success")
		job := CreatePendingJob(t, ctx, stack, fix, 1_067_320)
		id, err := store.PgtypeToUUID(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		jobIDs = append(jobIDs, id)
	}

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(workerN int) {
			defer wg.Done()
			workerID := fmt.Sprintf("concurrent-worker-%d", workerN)
			for i := 0; i < 30; i++ {
				_ = stack.Processor.ProcessOne(ctx, workerID)
			}
		}(w)
	}
	wg.Wait()

	for _, jobID := range jobIDs {
		job, err := JobByID(ctx, pool, jobID)
		if err != nil {
			t.Fatalf("load job %s: %v", jobID, err)
		}
		if job.Status != sqlc.SettlementJobStatusSucceeded {
			t.Fatalf("job %s expected succeeded, got %s", jobID, job.Status)
		}
		if job.PayoutReference == nil {
			t.Fatalf("job %s missing payout reference", jobID)
		}

		lines, err := LedgerLineCount(ctx, pool, jobID)
		if err != nil {
			t.Fatal(err)
		}
		if lines != 4 {
			t.Fatalf("job %s expected 4 ledger lines, got %d", jobID, lines)
		}
	}

	var duplicateRefs int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT payout_reference FROM settlement_jobs
			WHERE id = ANY($1) AND payout_reference IS NOT NULL
			GROUP BY payout_reference HAVING count(*) > 1
		) d
	`, jobIDs).Scan(&duplicateRefs); err != nil {
		t.Fatal(err)
	}
	if duplicateRefs > 0 {
		t.Fatal("found duplicate payout references across fixture jobs")
	}
}

func TestTransientProfileEventuallySucceedsWithRetries(t *testing.T) {
	pool, ctx := RequireDB(t)
	stack := NewStack(t, pool)
	fix := SeedDueMaturity(t, pool, ctx, "transient_then_success")
	job := CreateJobForFixture(t, ctx, stack, fix)

	jobID, err := store.PgtypeToUUID(job.ID)
	if err != nil {
		t.Fatal(err)
	}

	final := ProcessUntilStatus(t, ctx, stack, jobID, sqlc.SettlementJobStatusSucceeded, "transient-e2e", 10*time.Second)
	if final.RetryCount < 2 {
		t.Fatalf("expected at least 2 retries before success, got retry_count=%d", final.RetryCount)
	}

	attempts, err := AttemptCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 payout attempts, got %d", attempts)
	}

	lines, err := LedgerLineCount(ctx, pool, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 4 {
		t.Fatalf("expected 4 ledger lines after success, got %d", lines)
	}
}
