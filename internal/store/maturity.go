package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// DueMaturity is a maturity schedule due for settlement with its investment terms.
type DueMaturity = sqlc.ListDueMaturitySchedulesRow

// MaturityRepository scans and updates maturity schedules.
type MaturityRepository interface {
	ListDue(ctx context.Context, q *sqlc.Queries) ([]DueMaturity, error)
	MarkSettled(ctx context.Context, q *sqlc.Queries, maturityScheduleID pgtype.UUID) (sqlc.MaturitySchedule, error)
}

// MaturityRepo implements MaturityRepository.
type MaturityRepo struct{}

// ListDue returns pending maturities at or past their due time, locked for enqueue.
func (MaturityRepo) ListDue(ctx context.Context, q *sqlc.Queries) ([]DueMaturity, error) {
	rows, err := q.ListDueMaturitySchedules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list due maturities: %w", err)
	}
	return rows, nil
}

// MarkSettled marks a maturity schedule as settled after successful payout.
func (MaturityRepo) MarkSettled(ctx context.Context, q *sqlc.Queries, maturityScheduleID pgtype.UUID) (sqlc.MaturitySchedule, error) {
	row, err := q.MarkMaturitySettled(ctx, maturityScheduleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.MaturitySchedule{}, ErrConflict
		}
		return sqlc.MaturitySchedule{}, fmt.Errorf("mark maturity settled: %w", err)
	}
	return row, nil
}
