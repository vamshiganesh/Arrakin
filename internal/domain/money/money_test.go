package money_test

import (
	"testing"

	"github.com/vamshiganesh/arrakin/internal/domain/money"
)

func TestProratedReturnOneYearEightPercent(t *testing.T) {
	// $10,000 principal at 8% for 365 days => $800 return
	got, err := money.ProratedReturn(money.Cents(1_000_000), 800, 365)
	if err != nil {
		t.Fatal(err)
	}
	if got != money.Cents(80_000) {
		t.Fatalf("expected 80000, got %d", got)
	}
}

func TestApplyBasisPointsRounding(t *testing.T) {
	got, err := money.ApplyBasisPoints(money.Cents(80_000), 100)
	if err != nil {
		t.Fatal(err)
	}
	if got != money.Cents(800) {
		t.Fatalf("expected 800, got %d", got)
	}
}

func TestValidatePrincipalRejectsZero(t *testing.T) {
	if err := money.ValidatePrincipal(0); err == nil {
		t.Fatal("expected error for zero principal")
	}
}
