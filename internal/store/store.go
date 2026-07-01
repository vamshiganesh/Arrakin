package store

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// Store wraps sqlc-generated queries for repository access.
type Store struct {
	Queries *sqlc.Queries
}

// New creates a Store backed by the shared connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{Queries: sqlc.New(pool)}
}
