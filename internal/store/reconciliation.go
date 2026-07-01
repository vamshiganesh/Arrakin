package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// ReconciliationRepository builds and stores reconciliation snapshots.
type ReconciliationRepository interface {
	CreateSnapshot(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error)
	GetLatest(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error)
}

// ReconciliationRepo implements ReconciliationRepository.
type ReconciliationRepo struct{}

// CreateSnapshot aggregates current settlement job totals and persists a snapshot.
func (ReconciliationRepo) CreateSnapshot(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error) {
	stats, err := q.AggregateSettlementJobStats(ctx)
	if err != nil {
		return sqlc.ReconciliationSnapshot{}, fmt.Errorf("aggregate settlement job stats: %w", err)
	}

	discrepancy := stats.ExpectedTotalCents - stats.SucceededTotalCents
	details, err := json.Marshal(map[string]any{
		"total_job_count":   stats.TotalJobCount,
		"processing_count":  stats.ProcessingCount,
		"pending_count":     stats.PendingCount,
		"failed_count":      stats.FailedCount,
		"dead_letter_count": stats.DeadLetterCount,
		"succeeded_count":   stats.SucceededCount,
	})
	if err != nil {
		return sqlc.ReconciliationSnapshot{}, fmt.Errorf("marshal reconciliation details: %w", err)
	}

	snapshot, err := q.CreateReconciliationSnapshot(ctx, sqlc.CreateReconciliationSnapshotParams{
		ExpectedJobCount:    stats.TotalJobCount,
		ExpectedTotalCents:  stats.ExpectedTotalCents,
		SucceededCount:      stats.SucceededCount,
		SucceededTotalCents: stats.SucceededTotalCents,
		PendingCount:        stats.PendingCount,
		FailedCount:         stats.FailedCount,
		DeadLetterCount:     stats.DeadLetterCount,
		DiscrepancyCents:    discrepancy,
		Details:             details,
	})
	if err != nil {
		return sqlc.ReconciliationSnapshot{}, fmt.Errorf("create reconciliation snapshot: %w", err)
	}
	return snapshot, nil
}

// GetLatest returns the most recent reconciliation snapshot.
func (ReconciliationRepo) GetLatest(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error) {
	snapshot, err := q.GetLatestReconciliationSnapshot(ctx)
	if err != nil {
		return sqlc.ReconciliationSnapshot{}, mapNotFound(err, "get latest reconciliation snapshot")
	}
	return snapshot, nil
}

func mapNotFound(err error, action string) error {
	if err == nil {
		return nil
	}
	if err.Error() == "no rows in result set" {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}
