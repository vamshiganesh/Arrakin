package calculator_test

import (
	"testing"

	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
	"github.com/vamshiganesh/arrakin/internal/settlement/calculator"
)

func TestCalculateDemoInvestment(t *testing.T) {
	svc, err := calculator.New(calculator.Config{
		PlatformFeeBPS:    100,
		WithholdingTaxBPS: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}

	breakdown, err := svc.Calculate(settlement.InvestmentTerms{
		PrincipalCents: 1_000_000,
		AnnualRateBPS:  800,
		TermDays:       365,
		Currency:       "USD",
	})
	if err != nil {
		t.Fatal(err)
	}

	if breakdown.GrossReturnCents.Int64() != 80_000 {
		t.Fatalf("gross return: got %d want 80000", breakdown.GrossReturnCents)
	}
	if breakdown.PlatformFeeCents.Int64() != 800 {
		t.Fatalf("platform fee: got %d want 800", breakdown.PlatformFeeCents)
	}
	if breakdown.WithholdingTaxCents.Int64() != 11_880 {
		t.Fatalf("withholding tax: got %d want 11880", breakdown.WithholdingTaxCents)
	}
	if breakdown.NetPayoutCents.Int64() != 1_067_320 {
		t.Fatalf("net payout: got %d want 1067320", breakdown.NetPayoutCents)
	}
}

func TestCalculateHalfYearTerm(t *testing.T) {
	svc, err := calculator.New(calculator.Config{
		PlatformFeeBPS:    100,
		WithholdingTaxBPS: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}

	breakdown, err := svc.Calculate(settlement.InvestmentTerms{
		PrincipalCents: 2_500_000,
		AnnualRateBPS:  750,
		TermDays:       180,
		Currency:       "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := breakdown.Validate(); err != nil {
		t.Fatalf("invalid breakdown: %v", err)
	}
	if breakdown.GrossReturnCents.Int64() <= 0 {
		t.Fatalf("expected positive return, got %d", breakdown.GrossReturnCents)
	}
}

func TestCalculateRejectsInvalidTerms(t *testing.T) {
	svc, err := calculator.New(calculator.Config{PlatformFeeBPS: 100, WithholdingTaxBPS: 1500})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Calculate(settlement.InvestmentTerms{
		PrincipalCents: 0,
		AnnualRateBPS:  800,
		TermDays:       365,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
