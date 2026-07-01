package ledger

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

const (
	accountTypeLiability = "liability"
	accountTypeAsset       = "asset"
	accountTypeRevenue     = "revenue"

	sideDebit  = "D"
	sideCredit = "C"

	platformFeeAccountCode      = "PLATFORM_FEE_REVENUE"
	platformFeeAccountName      = "Platform Fee Revenue"
	withholdingTaxAccountCode   = "WITHHOLDING_TAX_PAYABLE"
	withholdingTaxAccountName   = "Withholding Tax Payable"
)

// PostingService writes balanced immutable ledger entries for settlements.
type PostingService struct {
	ledger store.LedgerRepository
}

// NewPostingService creates a ledger posting service.
func NewPostingService(ledger store.LedgerRepository) *PostingService {
	return &PostingService{ledger: ledger}
}

// PostSettlementInput identifies a successful settlement posting.
type PostSettlementInput struct {
	JobID      uuid.UUID
	InvestorID uuid.UUID
	Breakdown  settlement.Breakdown
}

// PostSettlement writes a balanced double-entry posting for a settlement breakdown.
// entry_group_id is derived deterministically from job_id for idempotent re-entry detection.
func (s *PostingService) PostSettlement(
	ctx context.Context,
	q *sqlc.Queries,
	input PostSettlementInput,
) ([]sqlc.LedgerEntry, error) {
	if err := input.Breakdown.Validate(); err != nil {
		return nil, fmt.Errorf("post settlement: %w", err)
	}
	if input.JobID == uuid.Nil {
		return nil, fmt.Errorf("post settlement: job id is required")
	}
	if input.InvestorID == uuid.Nil {
		return nil, fmt.Errorf("post settlement: investor id is required")
	}

	jobID := store.UUIDToPgtype(input.JobID)
	existingGroup, err := s.ledger.GetEntryGroupID(ctx, q, jobID)
	if err == nil {
		return s.ledger.ListByJobID(ctx, q, jobID)
	}
	if err != nil && !store.IsNotFound(err) {
		return nil, fmt.Errorf("post settlement: %w", err)
	}
	_ = existingGroup

	entryGroupID := store.UUIDToPgtype(EntryGroupIDFromJob(input.JobID))
	lines, err := buildPostingLines(input.InvestorID, input.Breakdown)
	if err != nil {
		return nil, err
	}
	if err := validateBalanced(lines); err != nil {
		return nil, err
	}

	entries, err := s.ledger.PostEntries(ctx, q, jobID, entryGroupID, lines)
	if err != nil {
		return nil, fmt.Errorf("post settlement: %w", err)
	}
	return entries, nil
}

// EntryGroupIDFromJob derives a stable entry group id from a settlement job id.
func EntryGroupIDFromJob(jobID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("arrakin:settlement:"+jobID.String()))
}

func investorPayableCode(investorID uuid.UUID) string {
	return fmt.Sprintf("INVESTOR_PAYABLE:%s", investorID)
}

func investorWalletCode(investorID uuid.UUID) string {
	return fmt.Sprintf("INVESTOR_WALLET:%s", investorID)
}

func buildPostingLines(investorID uuid.UUID, b settlement.Breakdown) ([]store.LedgerLineInput, error) {
	currency := b.Currency
	totalObligation := b.TotalObligationCents().Int64()
	metadata, err := json.Marshal(map[string]any{
		"investor_id": investorID.String(),
		"currency":    currency,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ledger metadata: %w", err)
	}

	return []store.LedgerLineInput{
		{
			AccountCode: investorPayableCode(investorID),
			AccountName: "Investor Payable",
			AccountType: accountTypeLiability,
			Side:        sideDebit,
			AmountCents: totalObligation,
			Currency:    currency,
			Description: "Settlement: reduce investor payable",
			Metadata:    metadata,
		},
		{
			AccountCode: investorWalletCode(investorID),
			AccountName: "Investor Wallet",
			AccountType: accountTypeAsset,
			Side:        sideCredit,
			AmountCents: b.NetPayoutCents.Int64(),
			Currency:    currency,
			Description: "Settlement: credit investor wallet",
			Metadata:    metadata,
		},
		{
			AccountCode: platformFeeAccountCode,
			AccountName: platformFeeAccountName,
			AccountType: accountTypeRevenue,
			Side:        sideCredit,
			AmountCents: b.PlatformFeeCents.Int64(),
			Currency:    currency,
			Description: "Settlement: recognize platform fee",
			Metadata:    metadata,
		},
		{
			AccountCode: withholdingTaxAccountCode,
			AccountName: withholdingTaxAccountName,
			AccountType: accountTypeLiability,
			Side:        sideCredit,
			AmountCents: b.WithholdingTaxCents.Int64(),
			Currency:    currency,
			Description: "Settlement: withhold tax payable",
			Metadata:    metadata,
		},
	}, nil
}

func validateBalanced(lines []store.LedgerLineInput) error {
	var debits, credits int64
	for _, line := range lines {
		if line.AmountCents <= 0 {
			return fmt.Errorf("ledger line amount must be positive")
		}
		switch line.Side {
		case sideDebit:
			debits += line.AmountCents
		case sideCredit:
			credits += line.AmountCents
		default:
			return fmt.Errorf("invalid ledger side %q", line.Side)
		}
	}
	if debits != credits {
		return fmt.Errorf("unbalanced posting: debits=%d credits=%d", debits, credits)
	}
	return nil
}
