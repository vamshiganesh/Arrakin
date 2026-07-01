package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// ReserveIdempotencyParams reserves a new idempotency key before handling a request.
type ReserveIdempotencyParams struct {
	Scope       string
	Key         string
	RequestHash pgtype.Text
	ExpiresAt   time.Time
}

// IdempotencyRepository manages HTTP/admin idempotency key storage.
type IdempotencyRepository interface {
	GetActive(ctx context.Context, q *sqlc.Queries, scope, key string) (sqlc.IdempotencyKey, error)
	Reserve(ctx context.Context, q *sqlc.Queries, params ReserveIdempotencyParams) (sqlc.IdempotencyKey, error)
	Complete(ctx context.Context, q *sqlc.Queries, scope, key string, status int32, body []byte) (sqlc.IdempotencyKey, error)
}

// IdempotencyRepo implements IdempotencyRepository.
type IdempotencyRepo struct{}

// GetActive returns a non-expired idempotency record when present.
func (IdempotencyRepo) GetActive(ctx context.Context, q *sqlc.Queries, scope, key string) (sqlc.IdempotencyKey, error) {
	row, err := q.GetActiveIdempotencyKey(ctx, sqlc.GetActiveIdempotencyKeyParams{
		Scope: scope,
		Key:   key,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.IdempotencyKey{}, ErrNotFound
		}
		return sqlc.IdempotencyKey{}, fmt.Errorf("get active idempotency key: %w", err)
	}
	return row, nil
}

// Reserve inserts a new idempotency key reservation.
func (IdempotencyRepo) Reserve(ctx context.Context, q *sqlc.Queries, params ReserveIdempotencyParams) (sqlc.IdempotencyKey, error) {
	row, err := q.CreateIdempotencyKey(ctx, sqlc.CreateIdempotencyKeyParams{
		Key:         params.Key,
		Scope:       params.Scope,
		RequestHash: params.RequestHash,
		ExpiresAt:   pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			return sqlc.IdempotencyKey{}, ErrIdempotencyKeyExists
		}
		return sqlc.IdempotencyKey{}, fmt.Errorf("create idempotency key: %w", err)
	}
	return row, nil
}

// Complete stores the response for a reserved idempotency key.
func (IdempotencyRepo) Complete(ctx context.Context, q *sqlc.Queries, scope, key string, status int32, body []byte) (sqlc.IdempotencyKey, error) {
	row, err := q.CompleteIdempotencyKey(ctx, sqlc.CompleteIdempotencyKeyParams{
		Scope:          scope,
		Key:            key,
		ResponseStatus: pgtype.Int4{Int32: status, Valid: true},
		ResponseBody:   body,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.IdempotencyKey{}, ErrNotFound
		}
		return sqlc.IdempotencyKey{}, fmt.Errorf("complete idempotency key: %w", err)
	}
	return row, nil
}
