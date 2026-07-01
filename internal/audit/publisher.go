package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// Well-known audit actions for settlement lifecycle events.
const (
	ActionSettlementJobCreated   = "settlement_job.created"
	ActionSettlementJobClaimed   = "settlement_job.claimed"
	ActionSettlementJobSucceeded = "settlement_job.succeeded"
	ActionSettlementJobFailed    = "settlement_job.failed"
	ActionSettlementJobDeadLetter = "settlement_job.dead_letter"
	ActionSettlementJobReplayed  = "settlement_job.replayed"
	ActionSettlementJobLeaseExpired = "settlement_job.lease_expired"
	ActionLedgerPosted           = "ledger.posted"
	ActionMaturitySettled        = "maturity.settled"
)

const entityTypeSettlementJob = "settlement_job"
const entityTypeMaturitySchedule = "maturity_schedule"
const entityTypeLedgerEntryGroup = "ledger_entry_group"

// Publisher records append-only audit events.
type Publisher struct {
	repo store.AuditRepository
}

// NewPublisher creates an audit publisher.
func NewPublisher(repo store.AuditRepository) *Publisher {
	return &Publisher{repo: repo}
}

// EventInput is the caller-facing audit payload.
type EventInput struct {
	ActorType     sqlc.AuditActorType
	ActorID       string
	Action        string
	EntityType    string
	EntityID      uuid.UUID
	Payload       map[string]any
	CorrelationID string
	OccurredAt    time.Time
}

// Publish writes an audit event inside the caller transaction.
func (p *Publisher) Publish(ctx context.Context, q *sqlc.Queries, input EventInput) (sqlc.AuditEvent, error) {
	if input.Action == "" {
		return sqlc.AuditEvent{}, fmt.Errorf("audit: action is required")
	}
	if input.EntityType == "" {
		return sqlc.AuditEvent{}, fmt.Errorf("audit: entity type is required")
	}
	if input.EntityID == uuid.Nil {
		return sqlc.AuditEvent{}, fmt.Errorf("audit: entity id is required")
	}

	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return sqlc.AuditEvent{}, fmt.Errorf("audit: marshal payload: %w", err)
	}

	event, err := p.repo.Record(ctx, q, store.AuditEventInput{
		ActorType:     input.ActorType,
		ActorID:       input.ActorID,
		Action:        input.Action,
		EntityType:    input.EntityType,
		EntityID:      store.UUIDToPgtype(input.EntityID),
		Payload:       body,
		CorrelationID: input.CorrelationID,
	})
	if err != nil {
		return sqlc.AuditEvent{}, fmt.Errorf("audit: %w", err)
	}
	return event, nil
}

// PublishJobTransition records a settlement job state transition.
func (p *Publisher) PublishJobTransition(
	ctx context.Context,
	q *sqlc.Queries,
	jobID uuid.UUID,
	action string,
	correlationID string,
	details map[string]any,
) (sqlc.AuditEvent, error) {
	return p.Publish(ctx, q, EventInput{
		ActorType:     sqlc.AuditActorTypeSystem,
		ActorID:       "settlement-engine",
		Action:        action,
		EntityType:    entityTypeSettlementJob,
		EntityID:      jobID,
		Payload:       details,
		CorrelationID: correlationID,
	})
}

// PublishLedgerPosted records a successful ledger posting for a settlement job.
func (p *Publisher) PublishLedgerPosted(
	ctx context.Context,
	q *sqlc.Queries,
	jobID uuid.UUID,
	entryGroupID uuid.UUID,
	correlationID string,
	lineCount int,
) (sqlc.AuditEvent, error) {
	return p.Publish(ctx, q, EventInput{
		ActorType:     sqlc.AuditActorTypeSystem,
		ActorID:       "ledger-service",
		Action:        ActionLedgerPosted,
		EntityType:    entityTypeLedgerEntryGroup,
		EntityID:      entryGroupID,
		CorrelationID: correlationID,
		Payload: map[string]any{
			"settlement_job_id": jobID.String(),
			"entry_group_id":    entryGroupID.String(),
			"line_count":        lineCount,
		},
	})
}
