package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// AuditEventInput captures one append-only audit record.
type AuditEventInput struct {
	ActorType     sqlc.AuditActorType
	ActorID       string
	Action        string
	EntityType    string
	EntityID      pgtype.UUID
	Payload       []byte
	CorrelationID string
}

// AuditRepository records audit events.
type AuditRepository interface {
	Record(ctx context.Context, q *sqlc.Queries, input AuditEventInput) (sqlc.AuditEvent, error)
	ListByEntity(ctx context.Context, q *sqlc.Queries, entityType string, entityID pgtype.UUID, limit int32) ([]sqlc.AuditEvent, error)
}

// AuditRepo implements AuditRepository.
type AuditRepo struct{}

// Record appends an audit event.
func (AuditRepo) Record(ctx context.Context, q *sqlc.Queries, input AuditEventInput) (sqlc.AuditEvent, error) {
	event, err := q.CreateAuditEvent(ctx, sqlc.CreateAuditEventParams{
		ActorType:     input.ActorType,
		ActorID:       input.ActorID,
		Action:        input.Action,
		EntityType:    input.EntityType,
		EntityID:      input.EntityID,
		Payload:       input.Payload,
		CorrelationID: input.CorrelationID,
	})
	if err != nil {
		return sqlc.AuditEvent{}, fmt.Errorf("create audit event: %w", err)
	}
	return event, nil
}

// ListByEntity returns recent audit events for an entity.
func (AuditRepo) ListByEntity(ctx context.Context, q *sqlc.Queries, entityType string, entityID pgtype.UUID, limit int32) ([]sqlc.AuditEvent, error) {
	events, err := q.ListAuditEventsByEntity(ctx, sqlc.ListAuditEventsByEntityParams{
		EntityType: entityType,
		EntityID:   entityID,
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	return events, nil
}

// IsNotFound reports whether err is a store not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}
