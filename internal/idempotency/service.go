package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// StoredResponse is a replayable HTTP response captured for an idempotency key.
type StoredResponse struct {
	StatusCode int
	Body       json.RawMessage
}

// State describes the lifecycle of an idempotency key lookup.
type State int

const (
	StateNotFound State = iota
	StateInProgress
	StateCompleted
)

// Service coordinates HTTP/admin idempotency key lifecycle.
type Service struct {
	repo store.IdempotencyRepository
	ttl  time.Duration
}

// NewService creates an idempotency service with the given key TTL.
func NewService(repo store.IdempotencyRepository, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Service{repo: repo, ttl: ttl}
}

func (s *Service) lookupState(ctx context.Context, q *sqlc.Queries, scope, key string) (State, StoredResponse, error) {
	row, err := s.repo.GetActive(ctx, q, scope, key)
	if err != nil {
		if store.IsNotFound(err) {
			return StateNotFound, StoredResponse{}, nil
		}
		return StateNotFound, StoredResponse{}, fmt.Errorf("idempotency lookup: %w", err)
	}
	if row.ResponseStatus == nil {
		return StateInProgress, StoredResponse{}, nil
	}
	return StateCompleted, StoredResponse{
		StatusCode: int(*row.ResponseStatus),
		Body:       row.ResponseBody,
	}, nil
}

// Lookup returns a stored response when an active completed idempotency key exists.
func (s *Service) Lookup(ctx context.Context, q *sqlc.Queries, scope, key string) (StoredResponse, bool, error) {
	state, response, err := s.lookupState(ctx, q, scope, key)
	if err != nil {
		return StoredResponse{}, false, err
	}
	if state != StateCompleted {
		return StoredResponse{}, false, nil
	}
	return response, true, nil
}

// Reserve claims an idempotency key before executing a mutating handler.
func (s *Service) Reserve(ctx context.Context, q *sqlc.Queries, scope, key, requestHash string) error {
	state, _, err := s.lookupState(ctx, q, scope, key)
	if err != nil {
		return err
	}
	if state != StateNotFound {
		return store.ErrIdempotencyKeyExists
	}

	var hash pgtype.Text
	if requestHash != "" {
		hash = pgtype.Text{String: requestHash, Valid: true}
	}
	_, err = s.repo.Reserve(ctx, q, store.ReserveIdempotencyParams{
		Scope:       scope,
		Key:         key,
		RequestHash: hash,
		ExpiresAt:   time.Now().Add(s.ttl),
	})
	if err != nil {
		if errors.Is(err, store.ErrIdempotencyKeyExists) {
			return err
		}
		return fmt.Errorf("idempotency reserve: %w", err)
	}
	return nil
}

// Complete stores the handler response for future replays.
func (s *Service) Complete(ctx context.Context, q *sqlc.Queries, scope, key string, response StoredResponse) error {
	_, err := s.repo.Complete(ctx, q, scope, key, int32(response.StatusCode), response.Body)
	if err != nil {
		return fmt.Errorf("idempotency complete: %w", err)
	}
	return nil
}

// Handler is a mutating operation whose response should be stored for replay.
type Handler func(ctx context.Context) (StoredResponse, error)

// Execute reserves a key, runs fn, and stores the response.
// When the key already exists with a completed response, the stored response is returned.
// When the key exists but is not yet completed, ErrKeyInProgress is returned.
func (s *Service) Execute(
	ctx context.Context,
	q *sqlc.Queries,
	scope, key, requestHash string,
	fn Handler,
) (StoredResponse, bool, error) {
	state, stored, err := s.lookupState(ctx, q, scope, key)
	if err != nil {
		return StoredResponse{}, false, err
	}
	switch state {
	case StateCompleted:
		return stored, true, nil
	case StateInProgress:
		return StoredResponse{}, false, ErrKeyInProgress
	}

	if err := s.Reserve(ctx, q, scope, key, requestHash); err != nil {
		if errors.Is(err, store.ErrIdempotencyKeyExists) {
			state, stored, lookupErr := s.lookupState(ctx, q, scope, key)
			if lookupErr != nil {
				return StoredResponse{}, false, lookupErr
			}
			switch state {
			case StateCompleted:
				return stored, true, nil
			case StateInProgress:
				return StoredResponse{}, false, ErrKeyInProgress
			default:
				return StoredResponse{}, false, err
			}
		}
		return StoredResponse{}, false, err
	}

	response, err := fn(ctx)
	if err != nil {
		return StoredResponse{}, false, err
	}
	if err := s.Complete(ctx, q, scope, key, response); err != nil {
		return StoredResponse{}, false, err
	}
	return response, false, nil
}
