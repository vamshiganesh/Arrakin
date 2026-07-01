package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when a requested row does not exist.
	ErrNotFound = errors.New("store: not found")

	// ErrNoJobAvailable is returned when a worker claim finds no eligible jobs.
	ErrNoJobAvailable = errors.New("store: no settlement job available to claim")

	// ErrConflict is returned when a state transition is invalid for the current row.
	ErrConflict = errors.New("store: state conflict")

	// ErrIdempotencyKeyExists is returned when an idempotency key is already reserved.
	ErrIdempotencyKeyExists = errors.New("store: idempotency key already exists")

	// ErrLedgerAlreadyPosted is returned when a settlement job already has ledger entries.
	ErrLedgerAlreadyPosted = errors.New("store: ledger already posted for job")
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
