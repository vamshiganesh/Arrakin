package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// LedgerService exposes ledger read APIs.
type LedgerService struct {
	store *store.Store
	repos store.Repositories
}

// NewLedgerService creates a ledger API service.
func NewLedgerService(st *store.Store) *LedgerService {
	return &LedgerService{
		store: st,
		repos: st.Repos(),
	}
}

// ListEntriesFilter holds ledger list filters.
type ListEntriesFilter struct {
	SettlementJobID *uuid.UUID
	AccountCode     *string
	FromTime        *time.Time
	ToTime          *time.Time
	CursorTime      pgtype.Timestamptz
	CursorID        pgtype.UUID
	Limit           int32
}

// ListEntries returns ledger entries matching filters.
func (s *LedgerService) ListEntries(ctx context.Context, filter ListEntriesFilter) ([]sqlc.LedgerEntry, error) {
	storeFilter := store.ListLedgerEntriesFilter{
		AccountCode: filter.AccountCode,
		CursorTime:  filter.CursorTime,
		CursorID:    filter.CursorID,
		Limit:       filter.Limit,
	}
	if filter.SettlementJobID != nil {
		storeFilter.SettlementJobID = store.UUIDToPgtype(*filter.SettlementJobID)
	}
	if filter.FromTime != nil {
		storeFilter.FromTime = pgtype.Timestamptz{Time: *filter.FromTime, Valid: true}
	}
	if filter.ToTime != nil {
		storeFilter.ToTime = pgtype.Timestamptz{Time: *filter.ToTime, Valid: true}
	}
	return s.repos.Ledger.List(ctx, s.store.Queries(), storeFilter)
}

// ListAccounts returns the chart of accounts.
func (s *LedgerService) ListAccounts(ctx context.Context) ([]sqlc.LedgerAccount, error) {
	return s.repos.Ledger.ListAccounts(ctx, s.store.Queries())
}
