package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// LedgerLineInput is one side of a double-entry posting.
type LedgerLineInput struct {
	AccountCode string
	AccountName string
	AccountType string
	Side        string
	AmountCents int64
	Currency    string
	Description string
	Metadata    []byte
}

// LedgerRepository writes immutable ledger entries.
type LedgerRepository interface {
	GetEntryGroupID(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) (pgtype.UUID, error)
	PostEntries(ctx context.Context, q *sqlc.Queries, jobID, entryGroupID pgtype.UUID, lines []LedgerLineInput) ([]sqlc.LedgerEntry, error)
	ListByJobID(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) ([]sqlc.LedgerEntry, error)
	List(ctx context.Context, q *sqlc.Queries, filter ListLedgerEntriesFilter) ([]sqlc.LedgerEntry, error)
	ListAccounts(ctx context.Context, q *sqlc.Queries) ([]sqlc.LedgerAccount, error)
}

// ListLedgerEntriesFilter controls ledger entry list queries.
type ListLedgerEntriesFilter struct {
	SettlementJobID pgtype.UUID
	AccountCode     *string
	FromTime        pgtype.Timestamptz
	ToTime          pgtype.Timestamptz
	CursorTime      pgtype.Timestamptz
	CursorID        pgtype.UUID
	Limit           int32
}

// LedgerRepo implements LedgerRepository.
type LedgerRepo struct{}

// GetEntryGroupID returns an existing posting group for a job, if any.
func (LedgerRepo) GetEntryGroupID(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) (pgtype.UUID, error) {
	groupID, err := q.GetLedgerEntryGroupIDByJobID(ctx, jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, ErrNotFound
		}
		return pgtype.UUID{}, fmt.Errorf("get ledger entry group: %w", err)
	}
	return groupID, nil
}

// PostEntries writes immutable ledger lines for a settlement job inside the caller transaction.
func (LedgerRepo) PostEntries(ctx context.Context, q *sqlc.Queries, jobID, entryGroupID pgtype.UUID, lines []LedgerLineInput) ([]sqlc.LedgerEntry, error) {
	existing, err := q.GetLedgerEntryGroupIDByJobID(ctx, jobID)
	if err == nil {
		id, convErr := PgtypeToUUID(existing)
		if convErr != nil {
			return nil, convErr
		}
		return nil, fmt.Errorf("%w: group %s", ErrLedgerAlreadyPosted, id)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing ledger posting: %w", err)
	}

	entries := make([]sqlc.LedgerEntry, 0, len(lines))
	for _, line := range lines {
		account, err := q.UpsertLedgerAccount(ctx, sqlc.UpsertLedgerAccountParams{
			Code:        line.AccountCode,
			Name:        line.AccountName,
			AccountType: line.AccountType,
		})
		if err != nil {
			return nil, fmt.Errorf("upsert ledger account %s: %w", line.AccountCode, err)
		}

		entry, err := q.InsertLedgerEntry(ctx, sqlc.InsertLedgerEntryParams{
			EntryGroupID:    entryGroupID,
			SettlementJobID: jobID,
			AccountID:       account.ID,
			Side:            line.Side,
			AmountCents:     line.AmountCents,
			Currency:        line.Currency,
			Description:     line.Description,
			Metadata:        line.Metadata,
		})
		if err != nil {
			return nil, fmt.Errorf("insert ledger entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// ListByJobID returns ledger lines for a settlement job.
func (LedgerRepo) ListByJobID(ctx context.Context, q *sqlc.Queries, jobID pgtype.UUID) ([]sqlc.LedgerEntry, error) {
	entries, err := q.ListLedgerEntriesByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	return entries, nil
}

// List returns ledger entries matching optional filters with cursor pagination.
func (LedgerRepo) List(ctx context.Context, q *sqlc.Queries, filter ListLedgerEntriesFilter) ([]sqlc.LedgerEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	entries, err := q.ListLedgerEntries(ctx, sqlc.ListLedgerEntriesParams{
		SettlementJobID: filter.SettlementJobID,
		AccountCode:     filter.AccountCode,
		FromTime:        filter.FromTime,
		ToTime:          filter.ToTime,
		CursorTime:      filter.CursorTime,
		CursorID:        filter.CursorID,
		LimitVal:        limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	return entries, nil
}

// ListAccounts returns all ledger accounts ordered by code.
func (LedgerRepo) ListAccounts(ctx context.Context, q *sqlc.Queries) ([]sqlc.LedgerAccount, error) {
	accounts, err := q.ListLedgerAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ledger accounts: %w", err)
	}
	return accounts, nil
}
