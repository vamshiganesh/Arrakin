package ledger_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/domain/money"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
	"github.com/vamshiganesh/arrakin/internal/ledger"
)

func TestEntryGroupIDFromJobDeterministic(t *testing.T) {
	jobID := uuid.MustParse("b2000001-0002-4002-8002-000000000001")
	a := ledger.EntryGroupIDFromJob(jobID)
	b := ledger.EntryGroupIDFromJob(jobID)
	if a != b {
		t.Fatalf("expected deterministic group id")
	}
}

func TestPostingBreakdownBalancesDebitsAndCredits(t *testing.T) {
	breakdown := settlement.Breakdown{
		PrincipalCents:      money.Cents(1_000_000),
		GrossReturnCents:    money.Cents(80_000),
		PlatformFeeCents:    money.Cents(800),
		WithholdingTaxCents: money.Cents(11_880),
		NetPayoutCents:      money.Cents(1_067_320),
		Currency:            "USD",
	}
	if err := breakdown.Validate(); err != nil {
		t.Fatal(err)
	}

	// Posting is exercised via integration; here we verify the breakdown identity used by ledger.
	if breakdown.TotalObligationCents() != money.Cents(1_080_000) {
		t.Fatalf("debit total mismatch")
	}
	credits := breakdown.NetPayoutCents.Add(breakdown.PlatformFeeCents).Add(breakdown.WithholdingTaxCents)
	if credits != breakdown.TotalObligationCents() {
		t.Fatalf("credit total %d != debit total %d", credits, breakdown.TotalObligationCents())
	}
}
