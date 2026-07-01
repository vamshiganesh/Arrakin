package money

import (
	"fmt"
	"math"
)

const (
	// BasisPointsDenominator is 100% expressed in basis points.
	BasisPointsDenominator = 10_000

	// DaysPerYear is the day count used for simple interest proration.
	DaysPerYear = 365
)

// Cents represents a monetary amount in minor units (e.g. USD cents).
// All settlement math uses this type to avoid floating-point error.
type Cents int64

// Int64 returns the raw cent value.
func (c Cents) Int64() int64 {
	return int64(c)
}

// IsPositive reports whether the amount is strictly greater than zero.
func (c Cents) IsPositive() bool {
	return c > 0
}

// IsNonNegative reports whether the amount is zero or positive.
func (c Cents) IsNonNegative() bool {
	return c >= 0
}

// Add returns the sum of two amounts.
func (c Cents) Add(other Cents) Cents {
	return c + other
}

// Sub returns the difference; callers must ensure the result is non-negative when required.
func (c Cents) Sub(other Cents) Cents {
	return c - other
}

// ValidatePrincipal ensures principal is a valid investment amount.
func ValidatePrincipal(cents int64) error {
	if cents <= 0 {
		return fmt.Errorf("principal must be positive, got %d", cents)
	}
	if cents > math.MaxInt64/10000 {
		return fmt.Errorf("principal exceeds safe calculation bounds")
	}
	return nil
}

// ValidateBasisPoints ensures a rate or fee is within 0-100%.
func ValidateBasisPoints(bps int) error {
	if bps < 0 || bps > BasisPointsDenominator {
		return fmt.Errorf("basis points must be between 0 and %d, got %d", BasisPointsDenominator, bps)
	}
	return nil
}

// ValidateTermDays ensures term length is positive.
func ValidateTermDays(days int) error {
	if days <= 0 {
		return fmt.Errorf("term days must be positive, got %d", days)
	}
	return nil
}

// ApplyBasisPoints computes amount * bps / 10000 with half-up rounding.
func ApplyBasisPoints(amount Cents, bps int) (Cents, error) {
	if err := ValidateBasisPoints(bps); err != nil {
		return 0, err
	}
	if amount == 0 || bps == 0 {
		return 0, nil
	}
	n := int64(amount) * int64(bps)
	return Cents((n + BasisPointsDenominator/2) / BasisPointsDenominator), nil
}

// ProratedReturn computes simple interest: principal * annualRateBps * termDays / (10000 * 365).
func ProratedReturn(principal Cents, annualRateBps, termDays int) (Cents, error) {
	if err := ValidateBasisPoints(annualRateBps); err != nil {
		return 0, err
	}
	if err := ValidateTermDays(termDays); err != nil {
		return 0, err
	}
	if principal == 0 || annualRateBps == 0 {
		return 0, nil
	}

	numerator := int64(principal) * int64(annualRateBps) * int64(termDays)
	denominator := int64(BasisPointsDenominator * DaysPerYear)
	return Cents((numerator + denominator/2) / denominator), nil
}
