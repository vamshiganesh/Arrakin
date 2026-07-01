package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// CreateJobParams captures calculated settlement amounts for job creation.
type CreateJobParams struct {
	MaturityScheduleID  pgtype.UUID
	InvestmentID        pgtype.UUID
	IdempotencyKey      string
	PrincipalCents      int64
	GrossReturnCents    int64
	PlatformFeeCents    int64
	WithholdingTaxCents int64
	NetPayoutCents      int64
	MaxRetries          int32
}

// SettlementJobRepository manages settlement job persistence and lifecycle transitions.
type SettlementJobRepository interface {
	CreateIdempotent(ctx context.Context, q *sqlc.Queries, params CreateJobParams) (sqlc.SettlementJob, bool, error)
	Claim(ctx context.Context, q *sqlc.Queries, workerID string) (sqlc.SettlementJob, error)
	MarkSucceeded(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID, payoutReference string) (sqlc.SettlementJob, error)
	MarkFailedRetryable(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID, nextRetryAt time.Time, lastError string) (sqlc.SettlementJob, error)
	MarkDeadLetter(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID, reason, lastError string, class sqlc.ErrorClass) (sqlc.SettlementJob, error)
	ReplayDeadLetter(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) (sqlc.SettlementJob, error)
	ExpireStaleLeases(ctx context.Context, q *sqlc.Queries, olderThan time.Time) ([]sqlc.SettlementJob, error)
	GetByMaturityScheduleID(ctx context.Context, q *sqlc.Queries, maturityScheduleID pgtype.UUID) (sqlc.SettlementJob, error)
}

// SettlementJobRepo implements SettlementJobRepository.
type SettlementJobRepo struct{}

// CreateIdempotent inserts a settlement job or returns the existing row for the maturity.
func (SettlementJobRepo) CreateIdempotent(ctx context.Context, q *sqlc.Queries, params CreateJobParams) (sqlc.SettlementJob, bool, error) {
	existing, err := q.GetSettlementJobByMaturityScheduleID(ctx, params.MaturityScheduleID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SettlementJob{}, false, fmt.Errorf("get settlement job before create: %w", err)
	}

	job, err := q.CreateSettlementJob(ctx, sqlc.CreateSettlementJobParams{
		MaturityScheduleID:  params.MaturityScheduleID,
		InvestmentID:        params.InvestmentID,
		IdempotencyKey:      params.IdempotencyKey,
		PrincipalCents:      params.PrincipalCents,
		GrossReturnCents:    params.GrossReturnCents,
		PlatformFeeCents:    params.PlatformFeeCents,
		WithholdingTaxCents: params.WithholdingTaxCents,
		NetPayoutCents:      params.NetPayoutCents,
		MaxRetries:          params.MaxRetries,
	})
	if err != nil {
		if isUniqueViolation(err) {
			existing, getErr := q.GetSettlementJobByMaturityScheduleID(ctx, params.MaturityScheduleID)
			if getErr != nil {
				return sqlc.SettlementJob{}, false, fmt.Errorf("get settlement job after conflict: %w", getErr)
			}
			return existing, false, nil
		}
		return sqlc.SettlementJob{}, false, fmt.Errorf("create settlement job: %w", err)
	}
	return job, true, nil
}

// Claim atomically leases the next eligible settlement job for a worker.
func (SettlementJobRepo) Claim(ctx context.Context, q *sqlc.Queries, workerID string) (sqlc.SettlementJob, error) {
	job, err := q.ClaimSettlementJob(ctx, StringPtr(workerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.SettlementJob{}, ErrNoJobAvailable
		}
		return sqlc.SettlementJob{}, fmt.Errorf("claim settlement job: %w", err)
	}
	return job, nil
}

// MarkSucceeded transitions a processing job to succeeded with a payout reference.
func (SettlementJobRepo) MarkSucceeded(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID, payoutReference string) (sqlc.SettlementJob, error) {
	job, err := q.MarkJobSucceeded(ctx, sqlc.MarkJobSucceededParams{
		ID:              jobID,
		PayoutReference: StringPtr(payoutReference),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.SettlementJob{}, ErrConflict
		}
		return sqlc.SettlementJob{}, fmt.Errorf("mark job succeeded: %w", err)
	}
	return job, nil
}

// MarkFailedRetryable transitions a processing job to failed with retry metadata.
func (SettlementJobRepo) MarkFailedRetryable(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID, nextRetryAt time.Time, lastError string) (sqlc.SettlementJob, error) {
	job, err := q.MarkJobFailedRetryable(ctx, sqlc.MarkJobFailedRetryableParams{
		ID:          jobID,
		NextRetryAt: pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
		LastError:   StringPtr(lastError),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.SettlementJob{}, ErrConflict
		}
		return sqlc.SettlementJob{}, fmt.Errorf("mark job failed retryable: %w", err)
	}
	return job, nil
}

// MarkDeadLetter transitions a processing job to terminal dead-letter state.
func (SettlementJobRepo) MarkDeadLetter(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID, reason, lastError string, class sqlc.ErrorClass) (sqlc.SettlementJob, error) {
	job, err := q.MarkJobDeadLetter(ctx, sqlc.MarkJobDeadLetterParams{
		ID:               jobID,
		DeadLetterReason: StringPtr(reason),
		LastError:        StringPtr(lastError),
		ErrorClass:       ErrorClassPtr(class),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.SettlementJob{}, ErrConflict
		}
		return sqlc.SettlementJob{}, fmt.Errorf("mark job dead letter: %w", err)
	}
	return job, nil
}

// ReplayDeadLetter moves a dead-letter job back to pending for admin replay.
func (SettlementJobRepo) ReplayDeadLetter(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) (sqlc.SettlementJob, error) {
	job, err := q.ReplayDeadLetterJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.SettlementJob{}, ErrConflict
		}
		return sqlc.SettlementJob{}, fmt.Errorf("replay dead letter job: %w", err)
	}
	return job, nil
}

// ExpireStaleLeases resets processing jobs whose worker lease has expired.
func (SettlementJobRepo) ExpireStaleLeases(ctx context.Context, q *sqlc.Queries, olderThan time.Time) ([]sqlc.SettlementJob, error) {
	jobs, err := q.ExpireStaleProcessingJobs(ctx, pgtype.Timestamptz{Time: olderThan, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("expire stale processing jobs: %w", err)
	}
	return jobs, nil
}

// GetByMaturityScheduleID fetches the job for a maturity schedule.
func (SettlementJobRepo) GetByMaturityScheduleID(ctx context.Context, q *sqlc.Queries, maturityScheduleID pgtype.UUID) (sqlc.SettlementJob, error) {
	job, err := q.GetSettlementJobByMaturityScheduleID(ctx, maturityScheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.SettlementJob{}, ErrNotFound
		}
		return sqlc.SettlementJob{}, fmt.Errorf("get settlement job: %w", err)
	}
	return job, nil
}
