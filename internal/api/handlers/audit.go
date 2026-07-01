package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/api/dto"
	"github.com/vamshiganesh/arrakin/internal/api/services"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
)

// AuditHandler serves audit event APIs.
type AuditHandler struct {
	svc *services.AuditService
}

// NewAuditHandler constructs audit handlers.
func NewAuditHandler(svc *services.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// ListEvents godoc
// @Summary List audit events
// @Tags audit
// @Produce json
// @Param entity_type query string false "Entity type filter"
// @Param entity_id query string false "Entity UUID filter"
// @Param action query string false "Action filter"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} dto.AuditEventListResponse
// @Router /audit/events [get]
func (h *AuditHandler) ListEvents(c *gin.Context) {
	page, err := httpx.ParsePageParams(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	filter := services.ListEventsFilter{
		CursorTime: page.CursorTime,
		CursorID:   page.CursorID,
		Limit:      page.Limit,
	}
	if raw := c.Query("entity_type"); raw != "" {
		filter.EntityType = &raw
	}
	if raw := c.Query("entity_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(c, httpx.ErrValidation)
			return
		}
		filter.EntityID = &id
	}
	if raw := c.Query("action"); raw != "" {
		filter.Action = &raw
	}

	events, err := h.svc.ListEvents(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	items := make([]dto.AuditEventResponse, 0, len(events))
	for _, event := range events {
		mapped, err := dto.MapAuditEvent(event)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		items = append(items, mapped)
	}

	meta := httpx.PageMeta{Limit: page.Limit, HasMore: len(events) == int(page.Limit)}
	if meta.HasMore && len(events) > 0 {
		last := events[len(events)-1]
		ts, id, err := dto.AuditCursorFromEvent(last)
		if err == nil {
			cursor := httpx.NextCursor(ts, id)
			meta.NextCursor = &cursor
		}
	}

	c.JSON(http.StatusOK, dto.AuditEventListResponse{Items: items, Page: meta})
}
