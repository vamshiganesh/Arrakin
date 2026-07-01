package calculator

import (
	"fmt"
	"strings"

	"github.com/vamshiganesh/arrakin/internal/domain/money"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
)

// Config holds fee and tax rates applied during settlement calculation.
type Config struct {
	PlatformFeeBPS    int
	WithholdingTaxBPS int
}

// Service computes settlement breakdowns from investment terms.
type Service struct {
	cfg Config
}

// New creates a settlement calculator with the given fee/tax configuration.
func New(cfg Config) (*Service, error) {
	if err := money.ValidateBasisPoints(cfg.PlatformFeeBPS); err != nil {
		return nil, fmt.Errorf("calculator config: %w", err)
	}
	if err := money.ValidateBasisPoints(cfg.WithholdingTaxBPS); err != nil {
		return nil, fmt.Errorf("calculator config: %w", err)
	}
	return &Service{cfg: cfg}, nil
}

// Calculate derives the settlement breakdown for matured investment terms.
//
// Flow:
//  1. gross_return = principal prorated by annual rate and term
//  2. platform_fee = bps fee on gross return
//  3. withholding_tax = bps tax on (gross_return - platform_fee)
//  4. net_payout = principal + gross_return - platform_fee - withholding_tax
func (s *Service) Calculate(terms settlement.InvestmentTerms) (settlement.Breakdown, error) {
	if err := terms.Validate(); err != nil {
		return settlement.Breakdown{}, err
	}

	principal := money.Cents(terms.PrincipalCents)
	grossReturn, err := money.ProratedReturn(principal, terms.AnnualRateBPS, terms.TermDays)
	if err != nil {
		return settlement.Breakdown{}, fmt.Errorf("calculate gross return: %w", err)
	}

	platformFee, err := money.ApplyBasisPoints(grossReturn, s.cfg.PlatformFeeBPS)
	if err != nil {
		return settlement.Breakdown{}, fmt.Errorf("calculate platform fee: %w", err)
	}

	taxableReturn := grossReturn.Sub(platformFee)
	withholdingTax, err := money.ApplyBasisPoints(taxableReturn, s.cfg.WithholdingTaxBPS)
	if err != nil {
		return settlement.Breakdown{}, fmt.Errorf("calculate withholding tax: %w", err)
	}

	netPayout := principal.Add(grossReturn).Sub(platformFee).Sub(withholdingTax)

	breakdown := settlement.Breakdown{
		PrincipalCents:      principal,
		GrossReturnCents:    grossReturn,
		PlatformFeeCents:    platformFee,
		WithholdingTaxCents: withholdingTax,
		NetPayoutCents:      netPayout,
		Currency:            strings.ToUpper(strings.TrimSpace(terms.Currency)),
	}
	if err := breakdown.Validate(); err != nil {
		return settlement.Breakdown{}, err
	}
	return breakdown, nil
}
