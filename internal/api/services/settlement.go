package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// SettlementService exposes settlement job read and admin actions.
type SettlementService struct {
	store *store.Store
	repos store.Repositories
	audit *audit.Publisher
}

// NewSettlementService creates a settlement API service.
func NewSettlementService(st *store.Store, auditPub *audit.Publisher) *SettlementService {
	return &SettlementService{
		store: st,
		repos: st.Repos(),
		audit: auditPub,
	}
}

// ListJobsFilter holds list query filters.
type ListJobsFilter struct {
	Status       *sqlc.SettlementJobStatus
	InvestmentID *uuid.UUID
	CursorTime   pgtype.Timestamptz
	CursorID     pgtype.UUID
	Limit        int32
}

// ListJobs returns settlement jobs matching filters.
func (s *SettlementService) ListJobs(ctx context.Context, filter ListJobsFilter) ([]sqlc.SettlementJob, error) {
	storeFilter := store.ListSettlementJobsFilter{
		Status:     filter.Status,
		CursorTime: filter.CursorTime,
		CursorID:   filter.CursorID,
		Limit:      filter.Limit,
	}
	if filter.InvestmentID != nil {
		storeFilter.InvestmentID = store.UUIDToPgtype(*filter.InvestmentID)
	}
	return s.repos.SettlementJobs.List(ctx, s.store.Queries(), storeFilter)
}

// GetJob returns a settlement job by ID.
func (s *SettlementService) GetJob(ctx context.Context, jobID uuid.UUID) (sqlc.SettlementJob, error) {
	return s.repos.SettlementJobs.GetByID(ctx, s.store.Queries(), store.UUIDToPgtype(jobID))
}

// ListAttempts returns payout attempts for a job.
func (s *SettlementService) ListAttempts(ctx context.Context, jobID uuid.UUID) ([]sqlc.PayoutAttempt, error) {
	if _, err := s.GetJob(ctx, jobID); err != nil {
		return nil, err
	}
	return s.repos.PayoutAttempts.ListByJobID(ctx, s.store.Queries(), store.UUIDToPgtype(jobID))
}

// ReplayDeadLetter moves a dead-letter job back to pending.
func (s *SettlementService) ReplayDeadLetter(ctx context.Context, jobID uuid.UUID, actorID, correlationID string) (sqlc.SettlementJob, error) {
	var job sqlc.SettlementJob
	err := s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		existing, err := s.repos.SettlementJobs.GetByID(ctx, q, store.UUIDToPgtype(jobID))
		if err != nil {
			return err
		}
		if existing.Status != sqlc.SettlementJobStatusDeadLetter {
			return fmt.Errorf("%w: job must be dead_letter, current status is %s", store.ErrConflict, existing.Status)
		}

		job, err = s.repos.SettlementJobs.ReplayDeadLetter(ctx, q, store.UUIDToPgtype(jobID))
		if err != nil {
			return err
		}

		_, err = s.audit.Publish(ctx, q, audit.EventInput{
			ActorType:     sqlc.AuditActorTypeAdmin,
			ActorID:       actorID,
			Action:        audit.ActionSettlementJobReplayed,
			EntityType:    "settlement_job",
			EntityID:      jobID,
			CorrelationID: correlationID,
			Payload: map[string]any{
				"previous_status": string(sqlc.SettlementJobStatusDeadLetter),
				"new_status":      string(job.Status),
			},
		})
		return err
	})
	return job, err
}

// RequeueFailed moves a failed job back to pending for immediate retry.
func (s *SettlementService) RequeueFailed(ctx context.Context, jobID uuid.UUID, actorID, correlationID string) (sqlc.SettlementJob, error) {
	var job sqlc.SettlementJob
	err := s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		existing, err := s.repos.SettlementJobs.GetByID(ctx, q, store.UUIDToPgtype(jobID))
		if err != nil {
			return err
		}
		if existing.Status != sqlc.SettlementJobStatusFailed {
			return fmt.Errorf("%w: job must be failed, current status is %s", store.ErrConflict, existing.Status)
		}

		job, err = s.repos.SettlementJobs.RequeueFailed(ctx, q, store.UUIDToPgtype(jobID))
		if err != nil {
			return err
		}

		_, err = s.audit.Publish(ctx, q, audit.EventInput{
			ActorType:     sqlc.AuditActorTypeAdmin,
			ActorID:       actorID,
			Action:        "settlement_job.requeued",
			EntityType:    "settlement_job",
			EntityID:      jobID,
			CorrelationID: correlationID,
			Payload: map[string]any{
				"previous_status": string(sqlc.SettlementJobStatusFailed),
				"new_status":      string(job.Status),
			},
		})
		return err
	})
	return job, err
}
