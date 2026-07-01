package services

import (
	"context"

	"github.com/vamshiganesh/arrakin/internal/reconciliation"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// ReconciliationService exposes reconciliation APIs.
type ReconciliationService struct {
	store *store.Store
	svc   *reconciliation.Service
}

// NewReconciliationService creates a reconciliation API service.
func NewReconciliationService(st *store.Store, svc *reconciliation.Service) *ReconciliationService {
	return &ReconciliationService{store: st, svc: svc}
}

// SnapshotWithFlags pairs a snapshot with computed flags.
type SnapshotWithFlags struct {
	Snapshot sqlc.ReconciliationSnapshot
	Flags    []string
}

// RunSnapshot creates a new reconciliation snapshot with flags.
func (s *ReconciliationService) RunSnapshot(ctx context.Context) (SnapshotWithFlags, error) {
	var result SnapshotWithFlags
	err := s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		snapshot, err := s.svc.RunSnapshot(ctx, q)
		if err != nil {
			return err
		}
		flags, err := s.svc.FlagsForSnapshot(ctx, q, snapshot)
		if err != nil {
			return err
		}
		result = SnapshotWithFlags{Snapshot: snapshot, Flags: flags}
		return nil
	})
	return result, err
}

// GetLatest returns the latest snapshot with flags.
func (s *ReconciliationService) GetLatest(ctx context.Context) (SnapshotWithFlags, error) {
	snapshot, err := s.svc.GetLatest(ctx, s.store.Queries())
	if err != nil {
		return SnapshotWithFlags{}, err
	}
	flags, err := s.svc.FlagsForSnapshot(ctx, s.store.Queries(), snapshot)
	if err != nil {
		return SnapshotWithFlags{}, err
	}
	return SnapshotWithFlags{Snapshot: snapshot, Flags: flags}, nil
}

// List returns historical snapshots (flags computed per snapshot discrepancy + live counts).
func (s *ReconciliationService) List(ctx context.Context, filter store.ListReconciliationSnapshotsFilter) ([]SnapshotWithFlags, error) {
	snapshots, err := s.svc.List(ctx, s.store.Queries(), filter)
	if err != nil {
		return nil, err
	}
	liveFlags, err := s.svc.FlagsForSnapshot(ctx, s.store.Queries(), sqlc.ReconciliationSnapshot{})
	if err != nil {
		return nil, err
	}
	_ = liveFlags

	out := make([]SnapshotWithFlags, 0, len(snapshots))
	for _, snapshot := range snapshots {
		flags := flagsFromSnapshot(snapshot)
		out = append(out, SnapshotWithFlags{Snapshot: snapshot, Flags: flags})
	}
	return out, nil
}

func flagsFromSnapshot(snapshot sqlc.ReconciliationSnapshot) []string {
	flags := make([]string, 0, 1)
	if snapshot.DiscrepancyCents != 0 {
		flags = append(flags, reconciliation.FlagAmountMismatch)
	}
	if snapshot.DeadLetterCount > 0 || snapshot.FailedCount > 0 {
		// Historical snapshots retain operational signal via status counts.
	}
	return flags
}
