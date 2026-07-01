package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// Store provides database access, transaction boundaries, and repositories.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store backed by the shared connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool returns the underlying pgx pool.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// Queries returns a query handle bound to the connection pool (non-transactional).
func (s *Store) Queries() *sqlc.Queries {
	return sqlc.New(s.pool)
}

// TxFunc runs fn inside a database transaction. The transaction rolls back unless fn returns nil.
func (s *Store) WithTx(ctx context.Context, fn func(ctx context.Context, q *sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	q := sqlc.New(tx)
	if err := fn(ctx, q); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// WithTxOptions runs fn inside a transaction with explicit pgx options.
func (s *Store) WithTxOptions(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context, q *sqlc.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	q := sqlc.New(tx)
	if err := fn(ctx, q); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// Repositories groups typed repository accessors.
type Repositories struct {
	Maturities      MaturityRepository
	SettlementJobs  SettlementJobRepository
	PayoutAttempts  PayoutAttemptRepository
	Ledger          LedgerRepository
	Idempotency     IdempotencyRepository
	Reconciliation  ReconciliationRepository
	Audit           AuditRepository
}

// Repos returns the default repository implementations.
func (s *Store) Repos() Repositories {
	return Repositories{
		Maturities:     MaturityRepo{},
		SettlementJobs: SettlementJobRepo{},
		PayoutAttempts: PayoutAttemptRepo{},
		Ledger:         LedgerRepo{},
		Idempotency:    IdempotencyRepo{},
		Reconciliation: ReconciliationRepo{},
		Audit:          AuditRepo{},
	}
}
