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

// AdminHandler serves admin operational endpoints.
type AdminHandler struct {
	svc   *services.AdminService
	store *store.Store
	idem  *idempotency.Service
}

// NewAdminHandler constructs admin handlers.
func NewAdminHandler(svc *services.AdminService, st *store.Store, idem *idempotency.Service) *AdminHandler {
	return &AdminHandler{svc: svc, store: st, idem: idem}
}

// SchedulerTick godoc
// @Summary Manually trigger scheduler maturity scan
// @Tags admin
// @Produce json
// @Param Idempotency-Key header string false "Idempotency key"
// @Success 200 {object} dto.SchedulerTickResponse
// @Router /admin/scheduler/tick [post]
func (h *AdminHandler) SchedulerTick(c *gin.Context) {
	runIdempotent(c, h.store, h.idem, "admin.scheduler.tick", func(ctx context.Context) (int, any, error) {
		created, err := h.svc.TriggerSchedulerScan(ctx)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, dto.SchedulerTickResponse{JobsCreated: created}, nil
	})
}
