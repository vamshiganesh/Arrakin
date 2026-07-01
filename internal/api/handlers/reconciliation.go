package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vamshiganesh/arrakin/internal/api/dto"
	"github.com/vamshiganesh/arrakin/internal/api/services"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
	"github.com/vamshiganesh/arrakin/internal/store"
)

// ReconciliationHandler serves reconciliation APIs.
type ReconciliationHandler struct {
	svc   *services.ReconciliationService
	store *store.Store
	idem  *idempotency.Service
}

// NewReconciliationHandler constructs reconciliation handlers.
func NewReconciliationHandler(svc *services.ReconciliationService, st *store.Store, idem *idempotency.Service) *ReconciliationHandler {
	return &ReconciliationHandler{svc: svc, store: st, idem: idem}
}

// GetLatest godoc
// @Summary Get latest reconciliation snapshot
// @Tags reconciliation
// @Produce json
// @Success 200 {object} dto.ReconciliationResponse
// @Router /reconciliation/latest [get]
func (h *ReconciliationHandler) GetLatest(c *gin.Context) {
	result, err := h.svc.GetLatest(c.Request.Context())
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	mapped, err := dto.MapReconciliationSnapshot(result.Snapshot, result.Flags)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapped)
}

// Run godoc
// @Summary Run reconciliation snapshot now
// @Tags reconciliation
// @Produce json
// @Param Idempotency-Key header string false "Idempotency key"
// @Success 200 {object} dto.ReconciliationResponse
// @Router /reconciliation/run [post]
func (h *ReconciliationHandler) Run(c *gin.Context) {
	runIdempotent(c, h.store, h.idem, "reconciliation.run", func(ctx context.Context) (int, any, error) {
		result, err := h.svc.RunSnapshot(ctx)
		if err != nil {
			return 0, nil, err
		}
		mapped, err := dto.MapReconciliationSnapshot(result.Snapshot, result.Flags)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, mapped, nil
	})
}

// ListSnapshots godoc
// @Summary List reconciliation snapshots
// @Tags reconciliation
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} dto.ReconciliationListResponse
// @Router /reconciliation/snapshots [get]
func (h *ReconciliationHandler) ListSnapshots(c *gin.Context) {
	page, err := httpx.ParsePageParams(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	results, err := h.svc.List(c.Request.Context(), store.ListReconciliationSnapshotsFilter{
		CursorTime: page.CursorTime,
		CursorID:   page.CursorID,
		Limit:      page.Limit,
	})
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	items := make([]dto.ReconciliationResponse, 0, len(results))
	for _, result := range results {
		mapped, err := dto.MapReconciliationSnapshot(result.Snapshot, result.Flags)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		items = append(items, mapped)
	}

	meta := httpx.PageMeta{Limit: page.Limit, HasMore: len(results) == int(page.Limit)}
	if meta.HasMore && len(results) > 0 {
		last := results[len(results)-1].Snapshot
		ts, id, err := dto.ReconciliationCursorFromSnapshot(last)
		if err == nil {
			cursor := httpx.NextCursor(ts, id)
			meta.NextCursor = &cursor
		}
	}

	c.JSON(http.StatusOK, dto.ReconciliationListResponse{Items: items, Page: meta})
}
