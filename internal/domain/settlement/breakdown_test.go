package settlement_test

import (
	"testing"

	"github.com/vamshiganesh/arrakin/internal/domain/money"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
)

func TestBreakdownValidateBalancedEquation(t *testing.T) {
	b := settlement.Breakdown{
		PrincipalCents:      money.Cents(1_000_000),
		GrossReturnCents:    money.Cents(80_000),
		PlatformFeeCents:    money.Cents(800),
		WithholdingTaxCents: money.Cents(11_880),
		NetPayoutCents:      money.Cents(1_067_320),
		Currency:            "USD",
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("expected valid breakdown: %v", err)
	}
	if b.TotalObligationCents() != money.Cents(1_080_000) {
		t.Fatalf("unexpected obligation: %d", b.TotalObligationCents())
	}
}

func TestBreakdownValidateRejectsMismatch(t *testing.T) {
	b := settlement.Breakdown{
		PrincipalCents:      money.Cents(100),
		GrossReturnCents:    money.Cents(10),
		PlatformFeeCents:    money.Cents(1),
		WithholdingTaxCents: money.Cents(1),
		NetPayoutCents:      money.Cents(999),
		Currency:            "USD",
	}
	if err := b.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
