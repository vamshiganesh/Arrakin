package reconciliation

import (
	"context"

	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

const (
	FlagAmountMismatch  = "amount_mismatch"
	FlagMissingLedger   = "missing_ledger"
	FlagOrphanLedger    = "orphan_ledger"
	FlagStalePending    = "stale_pending"
)

// Service builds reconciliation views with discrepancy flags.
type Service struct {
	repo store.ReconciliationRepository
}

// New creates a reconciliation service.
func New(repo store.ReconciliationRepository) *Service {
	return &Service{repo: repo}
}

// RunSnapshot persists a new reconciliation snapshot.
func (s *Service) RunSnapshot(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error) {
	return s.repo.CreateSnapshot(ctx, q)
}

// GetLatest returns the most recent snapshot.
func (s *Service) GetLatest(ctx context.Context, q *sqlc.Queries) (sqlc.ReconciliationSnapshot, error) {
	return s.repo.GetLatest(ctx, q)
}

// List returns historical snapshots.
func (s *Service) List(ctx context.Context, q *sqlc.Queries, filter store.ListReconciliationSnapshotsFilter) ([]sqlc.ReconciliationSnapshot, error) {
	return s.repo.List(ctx, q, filter)
}

// FlagsForSnapshot computes discrepancy flags for a snapshot.
func (s *Service) FlagsForSnapshot(ctx context.Context, q *sqlc.Queries, snapshot sqlc.ReconciliationSnapshot) ([]string, error) {
	counts, err := s.repo.FlagCounts(ctx, q)
	if err != nil {
		return nil, err
	}

	flags := make([]string, 0, 4)
	if snapshot.DiscrepancyCents != 0 {
		flags = append(flags, FlagAmountMismatch)
	}
	if counts.MissingLedger > 0 {
		flags = append(flags, FlagMissingLedger)
	}
	if counts.OrphanLedger > 0 {
		flags = append(flags, FlagOrphanLedger)
	}
	if counts.StalePending > 0 {
		flags = append(flags, FlagStalePending)
	}
	return flags, nil
}
