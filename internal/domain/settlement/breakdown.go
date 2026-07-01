package settlement

import (
	"fmt"
	"strings"

	"github.com/vamshiganesh/arrakin/internal/domain/money"
)

// InvestmentTerms are the inputs required to calculate a maturity settlement.
type InvestmentTerms struct {
	PrincipalCents  int64
	AnnualRateBPS   int
	TermDays        int
	Currency        string
}

// Validate checks investment terms before calculation.
func (t InvestmentTerms) Validate() error {
	if err := money.ValidatePrincipal(t.PrincipalCents); err != nil {
		return fmt.Errorf("investment terms: %w", err)
	}
	if err := money.ValidateBasisPoints(t.AnnualRateBPS); err != nil {
		return fmt.Errorf("investment terms: %w", err)
	}
	if err := money.ValidateTermDays(t.TermDays); err != nil {
		return fmt.Errorf("investment terms: %w", err)
	}
	currency := strings.TrimSpace(t.Currency)
	if len(currency) != 3 {
		return fmt.Errorf("investment terms: currency must be a 3-letter code, got %q", t.Currency)
	}
	return nil
}

// Breakdown is the full settlement amount decomposition for one maturity.
type Breakdown struct {
	PrincipalCents      money.Cents
	GrossReturnCents    money.Cents
	PlatformFeeCents    money.Cents
	WithholdingTaxCents money.Cents
	NetPayoutCents      money.Cents
	Currency            string
}

// Validate ensures internal consistency of the breakdown.
func (b Breakdown) Validate() error {
	if !b.PrincipalCents.IsPositive() {
		return fmt.Errorf("breakdown: principal must be positive")
	}
	if !b.GrossReturnCents.IsNonNegative() {
		return fmt.Errorf("breakdown: gross return must be non-negative")
	}
	if !b.PlatformFeeCents.IsNonNegative() {
		return fmt.Errorf("breakdown: platform fee must be non-negative")
	}
	if !b.WithholdingTaxCents.IsNonNegative() {
		return fmt.Errorf("breakdown: withholding tax must be non-negative")
	}
	if !b.NetPayoutCents.IsPositive() {
		return fmt.Errorf("breakdown: net payout must be positive")
	}

	expected := b.PrincipalCents.Add(b.GrossReturnCents).Sub(b.PlatformFeeCents).Sub(b.WithholdingTaxCents)
	if b.NetPayoutCents != expected {
		return fmt.Errorf(
			"breakdown: net payout %d != principal + return - fee - tax (%d)",
			b.NetPayoutCents,
			expected,
		)
	}

	taxable := b.GrossReturnCents.Sub(b.PlatformFeeCents)
	if b.WithholdingTaxCents > taxable {
		return fmt.Errorf("breakdown: withholding tax exceeds taxable return")
	}

	currency := strings.TrimSpace(b.Currency)
	if len(currency) != 3 {
		return fmt.Errorf("breakdown: invalid currency %q", b.Currency)
	}
	return nil
}

// TotalObligationCents is principal plus gross return (investor payable before fee/tax split).
func (b Breakdown) TotalObligationCents() money.Cents {
	return b.PrincipalCents.Add(b.GrossReturnCents)
}
