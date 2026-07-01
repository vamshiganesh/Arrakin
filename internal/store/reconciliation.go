package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// ReconciliationRepository builds and stores reconciliation snapshots.
type ReconciliationRepository interface {
	CreateSnapshot(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error)
	GetLatest(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error)
	List(ctx context.Context, q *sqlc.Queries, filter ListReconciliationSnapshotsFilter) ([]sqlc.ReconciliationSnapshot, error)
	FlagCounts(ctx context.Context, q *sqlc.Queries) (ReconciliationFlagCounts, error)
}

// ListReconciliationSnapshotsFilter controls reconciliation snapshot list queries.
type ListReconciliationSnapshotsFilter struct {
	CursorTime pgtype.Timestamptz
	CursorID   pgtype.UUID
	Limit      int32
}

// ReconciliationFlagCounts holds inputs for discrepancy flag computation.
type ReconciliationFlagCounts struct {
	MissingLedger int32
	OrphanLedger  int32
	StalePending  int32
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
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.ReconciliationSnapshot{}, ErrNotFound
		}
		return sqlc.ReconciliationSnapshot{}, fmt.Errorf("get latest reconciliation snapshot: %w", err)
	}
	return snapshot, nil
}

// List returns historical reconciliation snapshots with cursor pagination.
func (ReconciliationRepo) List(ctx context.Context, q *sqlc.Queries, filter ListReconciliationSnapshotsFilter) ([]sqlc.ReconciliationSnapshot, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	snapshots, err := q.ListReconciliationSnapshots(ctx, sqlc.ListReconciliationSnapshotsParams{
		CursorTime: filter.CursorTime,
		CursorID:   filter.CursorID,
		LimitVal:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list reconciliation snapshots: %w", err)
	}
	return snapshots, nil
}

// FlagCounts returns counts used to compute reconciliation discrepancy flags.
func (ReconciliationRepo) FlagCounts(ctx context.Context, q *sqlc.Queries) (ReconciliationFlagCounts, error) {
	missing, err := q.CountSucceededJobsWithoutLedger(ctx)
	if err != nil {
		return ReconciliationFlagCounts{}, fmt.Errorf("count missing ledger: %w", err)
	}
	orphan, err := q.CountOrphanLedgerJobGroups(ctx)
	if err != nil {
		return ReconciliationFlagCounts{}, fmt.Errorf("count orphan ledger: %w", err)
	}
	stale, err := q.CountStalePendingJobs(ctx)
	if err != nil {
		return ReconciliationFlagCounts{}, fmt.Errorf("count stale pending: %w", err)
	}
	return ReconciliationFlagCounts{
		MissingLedger: missing,
		OrphanLedger:  orphan,
		StalePending:  stale,
	}, nil
}
