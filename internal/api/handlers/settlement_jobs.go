package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/api/dto"
	"github.com/vamshiganesh/arrakin/internal/api/services"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// SettlementHandler serves settlement job APIs.
type SettlementHandler struct {
	svc   *services.SettlementService
	store *store.Store
	idem  *idempotency.Service
}

// NewSettlementHandler constructs settlement handlers.
func NewSettlementHandler(svc *services.SettlementService, st *store.Store, idem *idempotency.Service) *SettlementHandler {
	return &SettlementHandler{svc: svc, store: st, idem: idem}
}

// ListJobs godoc
// @Summary List settlement jobs
// @Tags settlement-jobs
// @Produce json
// @Param status query string false "Job status filter"
// @Param investment_id query string false "Investment UUID filter"
// @Param limit query int false "Page size (max 200)"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} dto.SettlementJobListResponse
// @Router /settlement-jobs [get]
func (h *SettlementHandler) ListJobs(c *gin.Context) {
	page, err := httpx.ParsePageParams(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	filter := services.ListJobsFilter{
		CursorTime: page.CursorTime,
		CursorID:   page.CursorID,
		Limit:      page.Limit,
	}
	if raw := c.Query("status"); raw != "" {
		status := sqlc.SettlementJobStatus(raw)
		filter.Status = &status
	}
	if raw := c.Query("investment_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(c, httpx.ErrValidation)
			return
		}
		filter.InvestmentID = &id
	}

	jobs, err := h.svc.ListJobs(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	items := make([]dto.SettlementJobResponse, 0, len(jobs))
	for _, job := range jobs {
		mapped, err := dto.MapSettlementJob(job)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		items = append(items, mapped)
	}

	meta := httpx.PageMeta{Limit: page.Limit, HasMore: len(jobs) == int(page.Limit)}
	if meta.HasMore && len(jobs) > 0 {
		last := jobs[len(jobs)-1]
		ts, id, err := dto.JobCursorFromSettlementJob(last)
		if err == nil {
			cursor := httpx.NextCursor(ts, id)
			meta.NextCursor = &cursor
		}
	}

	c.JSON(http.StatusOK, dto.SettlementJobListResponse{Items: items, Page: meta})
}

// GetJob godoc
// @Summary Get settlement job by ID
// @Tags settlement-jobs
// @Produce json
// @Param id path string true "Job UUID"
// @Success 200 {object} dto.SettlementJobResponse
// @Router /settlement-jobs/{id} [get]
func (h *SettlementHandler) GetJob(c *gin.Context) {
	jobID, err := parseUUIDParam(c, "id")
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	job, err := h.svc.GetJob(c.Request.Context(), jobID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	mapped, err := dto.MapSettlementJob(job)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapped)
}

// ListAttempts godoc
// @Summary List payout attempts for a job
// @Tags settlement-jobs
// @Produce json
// @Param id path string true "Job UUID"
// @Success 200 {object} dto.PayoutAttemptListResponse
// @Router /settlement-jobs/{id}/attempts [get]
func (h *SettlementHandler) ListAttempts(c *gin.Context) {
	jobID, err := parseUUIDParam(c, "id")
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	attempts, err := h.svc.ListAttempts(c.Request.Context(), jobID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	items := make([]dto.PayoutAttemptResponse, 0, len(attempts))
	for _, attempt := range attempts {
		mapped, err := dto.MapPayoutAttempt(attempt)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		items = append(items, mapped)
	}
	c.JSON(http.StatusOK, dto.PayoutAttemptListResponse{Items: items})
}

// ReplayJob godoc
// @Summary Replay a dead-letter settlement job
// @Tags settlement-jobs
// @Produce json
// @Param id path string true "Job UUID"
// @Param Idempotency-Key header string false "Idempotency key"
// @Success 200 {object} dto.JobActionResponse
// @Router /settlement-jobs/{id}/replay [post]
func (h *SettlementHandler) ReplayJob(c *gin.Context) {
	jobID, err := parseUUIDParam(c, "id")
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	runIdempotent(c, h.store, h.idem, "settlement_job.replay:"+jobID.String(), func(ctx context.Context) (int, any, error) {
		job, err := h.svc.ReplayDeadLetter(ctx, jobID, actorID(c), httpx.RequestIDFromContext(c))
		if err != nil {
			return 0, nil, err
		}
		mapped, err := dto.MapSettlementJob(job)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, dto.JobActionResponse{Job: mapped}, nil
	})
}

// RequeueJob godoc
// @Summary Requeue a failed settlement job for immediate retry
// @Tags settlement-jobs
// @Produce json
// @Param id path string true "Job UUID"
// @Param Idempotency-Key header string false "Idempotency key"
// @Success 200 {object} dto.JobActionResponse
// @Router /settlement-jobs/{id}/requeue [post]
func (h *SettlementHandler) RequeueJob(c *gin.Context) {
	jobID, err := parseUUIDParam(c, "id")
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	runIdempotent(c, h.store, h.idem, "settlement_job.requeue:"+jobID.String(), func(ctx context.Context) (int, any, error) {
		job, err := h.svc.RequeueFailed(ctx, jobID, actorID(c), httpx.RequestIDFromContext(c))
		if err != nil {
			return 0, nil, err
		}
		mapped, err := dto.MapSettlementJob(job)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, dto.JobActionResponse{Job: mapped}, nil
	})
}
