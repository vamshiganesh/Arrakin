package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// AuditService exposes audit event queries.
type AuditService struct {
	store *store.Store
	repos store.Repositories
}

// NewAuditService creates an audit API service.
func NewAuditService(st *store.Store) *AuditService {
	return &AuditService{
		store: st,
		repos: st.Repos(),
	}
}

// ListEventsFilter holds audit list filters.
type ListEventsFilter struct {
	EntityType *string
	EntityID   *uuid.UUID
	Action     *string
	CursorTime pgtype.Timestamptz
	CursorID   pgtype.UUID
	Limit      int32
}

// ListEvents returns audit events matching filters.
func (s *AuditService) ListEvents(ctx context.Context, filter ListEventsFilter) ([]sqlc.AuditEvent, error) {
	storeFilter := store.ListAuditEventsFilter{
		EntityType: filter.EntityType,
		Action:     filter.Action,
		CursorTime: filter.CursorTime,
		CursorID:   filter.CursorID,
		Limit:      filter.Limit,
	}
	if filter.EntityID != nil {
		storeFilter.EntityID = store.UUIDToPgtype(*filter.EntityID)
	}
	return s.repos.Audit.List(ctx, s.store.Queries(), storeFilter)
}
