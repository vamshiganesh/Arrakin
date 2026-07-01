package reconciliation_test

import (
	"testing"

	"github.com/vamshiganesh/arrakin/internal/reconciliation"
)

func TestFlagConstants(t *testing.T) {
	flags := []string{
		reconciliation.FlagAmountMismatch,
		reconciliation.FlagMissingLedger,
		reconciliation.FlagOrphanLedger,
		reconciliation.FlagStalePending,
	}
	for _, flag := range flags {
		if flag == "" {
			t.Fatal("flag constant must not be empty")
		}
	}
}
