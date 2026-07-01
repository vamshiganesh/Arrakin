package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// PayoutAttemptRepository records payout execution attempts.
type PayoutAttemptRepository interface {
	Start(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) (sqlc.PayoutAttempt, error)
	MarkSucceeded(ctx context.Context, q *sqlc.Queries, attemptID pgtype.UUID, payoutReference string) (sqlc.PayoutAttempt, error)
	MarkFailed(ctx context.Context, q *sqlc.Queries, attemptID pgtype.UUID, message string, class sqlc.ErrorClass) (sqlc.PayoutAttempt, error)
	ListByJobID(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) ([]sqlc.PayoutAttempt, error)
}

// PayoutAttemptRepo implements PayoutAttemptRepository.
type PayoutAttemptRepo struct{}

// Start creates a new payout attempt with the next attempt number for the job.
func (PayoutAttemptRepo) Start(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) (sqlc.PayoutAttempt, error) {
	next, err := q.GetNextAttemptNumber(ctx, jobID)
	if err != nil {
		return sqlc.PayoutAttempt{}, fmt.Errorf("get next attempt number: %w", err)
	}

	attempt, err := q.CreatePayoutAttempt(ctx, sqlc.CreatePayoutAttemptParams{
		SettlementJobID: jobID,
		AttemptNumber:   next,
	})
	if err != nil {
		return sqlc.PayoutAttempt{}, fmt.Errorf("create payout attempt: %w", err)
	}
	return attempt, nil
}

// MarkSucceeded marks an in-flight attempt as succeeded.
func (PayoutAttemptRepo) MarkSucceeded(ctx context.Context, q *sqlc.Queries, attemptID pgtype.UUID, payoutReference string) (sqlc.PayoutAttempt, error) {
	attempt, err := q.FinishPayoutAttemptSuccess(ctx, sqlc.FinishPayoutAttemptSuccessParams{
		ID:              attemptID,
		PayoutReference: StringPtr(payoutReference),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.PayoutAttempt{}, ErrConflict
		}
		return sqlc.PayoutAttempt{}, fmt.Errorf("finish payout attempt success: %w", err)
	}
	return attempt, nil
}

// MarkFailed marks an in-flight attempt as failed.
func (PayoutAttemptRepo) MarkFailed(ctx context.Context, q *sqlc.Queries, attemptID pgtype.UUID, message string, class sqlc.ErrorClass) (sqlc.PayoutAttempt, error) {
	attempt, err := q.FinishPayoutAttemptFailure(ctx, sqlc.FinishPayoutAttemptFailureParams{
		ID:           attemptID,
		ErrorMessage: StringPtr(message),
		ErrorClass:   ErrorClassPtr(class),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.PayoutAttempt{}, ErrConflict
		}
		return sqlc.PayoutAttempt{}, fmt.Errorf("finish payout attempt failure: %w", err)
	}
	return attempt, nil
}

// ListByJobID returns attempt history for a settlement job.
func (PayoutAttemptRepo) ListByJobID(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) ([]sqlc.PayoutAttempt, error) {
	attempts, err := q.ListPayoutAttemptsByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("list payout attempts: %w", err)
	}
	return attempts, nil
}
