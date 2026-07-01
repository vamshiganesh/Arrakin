package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/domain/money"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
	"github.com/vamshiganesh/arrakin/internal/ledger"
	"github.com/vamshiganesh/arrakin/internal/platform/metrics"
	"github.com/vamshiganesh/arrakin/internal/settlement/payout"
	"github.com/vamshiganesh/arrakin/internal/settlement/retry"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// Processor executes one settlement job inside a database transaction.
type Processor struct {
	store   *store.Store
	repos   store.Repositories
	ledger  *ledger.PostingService
	audit   *audit.Publisher
	gateway payout.Gateway
	retry   retry.Policy
}

// NewProcessor creates a settlement job processor.
func NewProcessor(
	st *store.Store,
	ledgerSvc *ledger.PostingService,
	auditPub *audit.Publisher,
	gateway payout.Gateway,
	retryPolicy retry.Policy,
) *Processor {
	return &Processor{
		store:   st,
		repos:   st.Repos(),
		ledger:  ledgerSvc,
		audit:   auditPub,
		gateway: gateway,
		retry:   retryPolicy,
	}
}

// ProcessOne claims and processes a single job when available.
func (p *Processor) ProcessOne(ctx context.Context, workerID string) error {
	err := p.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		job, err := p.repos.SettlementJobs.Claim(ctx, q, workerID)
		if err != nil {
			return err
		}
		metrics.Global.JobsClaimed.Add(1)

		jobID, err := store.PgtypeToUUID(job.ID)
		if err != nil {
			return err
		}

		investment, err := q.GetInvestmentByID(ctx, job.InvestmentID)
		if err != nil {
			return fmt.Errorf("load investment: %w", err)
		}
		investorID, err := store.PgtypeToUUID(investment.InvestorID)
		if err != nil {
			return err
		}

		attempt, err := p.repos.PayoutAttempts.Start(ctx, q, job.ID)
		if err != nil {
			return fmt.Errorf("start payout attempt: %w", err)
		}

		if _, err := p.audit.PublishJobTransition(ctx, q, jobID, audit.ActionSettlementJobClaimed, workerID, map[string]any{
			"attempt_number": attempt.AttemptNumber,
		}); err != nil {
			return err
		}

		result := p.gateway.Execute(payout.ExecuteInput{
			JobID:             jobID,
			AttemptNumber:     attempt.AttemptNumber,
			SimulationProfile: investment.SimulationProfile,
			NetPayoutCents:    job.NetPayoutCents,
		})

		if result.Err == nil {
			return p.completeSuccess(ctx, q, job, attempt, investorID, jobID, investment.Currency, result.PayoutReference, workerID)
		}

		return p.handleFailure(ctx, q, job, attempt, jobID, workerID, result)
	})
	if errors.Is(err, store.ErrNoJobAvailable) {
		return err
	}
	if err != nil {
		slog.Error("worker process failed", "worker_id", workerID, "error", err)
	}
	return err
}

func (p *Processor) completeSuccess(
	ctx context.Context,
	q *sqlc.Queries,
	job sqlc.SettlementJob,
	attempt sqlc.PayoutAttempt,
	investorID, jobID uuid.UUID,
	payoutReference, workerID string,
) error {
	breakdown := settlement.Breakdown{
		PrincipalCents:      money.Cents(job.PrincipalCents),
		GrossReturnCents:    money.Cents(job.GrossReturnCents),
		PlatformFeeCents:    money.Cents(job.PlatformFeeCents),
		WithholdingTaxCents: money.Cents(job.WithholdingTaxCents),
		NetPayoutCents:      money.Cents(job.NetPayoutCents),
		Currency:            "USD",
	}
	if err := breakdown.Validate(); err != nil {
		return err
	}

	entries, err := p.ledger.PostSettlement(ctx, q, ledger.PostSettlementInput{
		JobID:      jobID,
		InvestorID: investorID,
		Breakdown:  breakdown,
	})
	if err != nil && !errors.Is(err, store.ErrLedgerAlreadyPosted) {
		return fmt.Errorf("post ledger: %w", err)
	}

	if _, err := p.repos.PayoutAttempts.MarkSucceeded(ctx, q, attempt.ID, payoutReference); err != nil {
		return err
	}
	if _, err := p.repos.SettlementJobs.MarkSucceeded(ctx, q, job.ID, payoutReference); err != nil {
		return err
	}
	if _, err := p.repos.Maturities.MarkSettled(ctx, q, job.MaturityScheduleID); err != nil {
		return err
	}

	groupID := ledger.EntryGroupIDFromJob(jobID)
	if _, err := p.audit.PublishLedgerPosted(ctx, q, jobID, groupID, workerID, len(entries)); err != nil {
		return err
	}
	if _, err := p.audit.PublishJobTransition(ctx, q, jobID, audit.ActionSettlementJobSucceeded, workerID, map[string]any{
		"payout_reference": payoutReference,
	}); err != nil {
		return err
	}

	metrics.Global.JobsSucceeded.Add(1)
	slog.Info("settlement job succeeded",
		"job_id", jobID,
		"worker_id", workerID,
		"payout_reference", payoutReference,
	)
	return nil
}

func (p *Processor) handleFailure(
	ctx context.Context,
	q *sqlc.Queries,
	job sqlc.SettlementJob,
	attempt sqlc.PayoutAttempt,
	jobID uuid.UUID,
	workerID string,
	result payout.Result,
) error {
	errorClass := result.ErrorClass
	if errorClass == "" {
		errorClass = sqlc.ErrorClassTransient
	}

	if _, err := p.repos.PayoutAttempts.MarkFailed(ctx, q, attempt.ID, result.Err.Error(), errorClass); err != nil {
		return err
	}

	if errorClass == sqlc.ErrorClassTerminal || job.RetryCount+1 >= job.MaxRetries {
		reason := "terminal payout failure"
		if errorClass != sqlc.ErrorClassTerminal {
			reason = "max retries exceeded"
		}
		if _, err := p.repos.SettlementJobs.MarkDeadLetter(ctx, q, job.ID, reason, result.Err.Error(), errorClass); err != nil {
			return err
		}
		if _, err := p.audit.PublishJobTransition(ctx, q, jobID, audit.ActionSettlementJobDeadLetter, workerID, map[string]any{
			"reason": reason,
			"error":  result.Err.Error(),
		}); err != nil {
			return err
		}
		metrics.Global.JobsDeadLettered.Add(1)
		slog.Warn("settlement job dead-lettered",
			"job_id", jobID,
			"worker_id", workerID,
			"reason", reason,
		)
		return nil
	}

	nextRetry := p.retry.NextRetryAt(job.RetryCount, time.Now())
	if _, err := p.repos.SettlementJobs.MarkFailedRetryable(ctx, q, job.ID, nextRetry, result.Err.Error()); err != nil {
		return err
	}
	if _, err := p.audit.PublishJobTransition(ctx, q, jobID, audit.ActionSettlementJobFailed, workerID, map[string]any{
		"retry_count":     job.RetryCount + 1,
		"next_retry_at":   nextRetry,
		"error":           result.Err.Error(),
		"attempt_number":  attempt.AttemptNumber,
	}); err != nil {
		return err
	}

	metrics.Global.JobsRetried.Add(1)
	slog.Warn("settlement job scheduled for retry",
		"job_id", jobID,
		"worker_id", workerID,
		"retry_count", job.RetryCount+1,
		"next_retry_at", nextRetry,
	)
	return nil
}
