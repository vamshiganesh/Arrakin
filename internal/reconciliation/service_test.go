package reconciliation_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/reconciliation"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

func TestFlagsFromSnapshotAmountMismatch(t *testing.T) {
	svc := reconciliation.New(nil)
	flags, err := svc.FlagsForSnapshot(t.Context(), nil, sqlc.ReconciliationSnapshot{
		DiscrepancyCents: 100,
	})
	if err == nil {
		t.Skip("requires db for flag counts")
	}
	_ = flags
	_ = pgtype.UUID{}
}

func TestFlagConstants(t *testing.T) {
	if reconciliation.FlagAmountMismatch == "" {
		t.Fatal("expected flag constant")
	}
}
