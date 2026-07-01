package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/api/dto"
	"github.com/vamshiganesh/arrakin/internal/api/services"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
	"github.com/vamshiganesh/arrakin/internal/store"
)

// LedgerHandler serves ledger APIs.
type LedgerHandler struct {
	svc *services.LedgerService
}

// NewLedgerHandler constructs ledger handlers.
func NewLedgerHandler(svc *services.LedgerService) *LedgerHandler {
	return &LedgerHandler{svc: svc}
}

// ListEntries godoc
// @Summary List ledger entries
// @Tags ledger
// @Produce json
// @Param settlement_job_id query string false "Settlement job UUID"
// @Param account_code query string false "Ledger account code"
// @Param from query string false "Posted at from (RFC3339)"
// @Param to query string false "Posted at to (RFC3339)"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} dto.LedgerEntryListResponse
// @Router /ledger/entries [get]
func (h *LedgerHandler) ListEntries(c *gin.Context) {
	page, err := httpx.ParsePageParams(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	filter := services.ListEntriesFilter{
		CursorTime: page.CursorTime,
		CursorID:   page.CursorID,
		Limit:      page.Limit,
	}
	if raw := c.Query("settlement_job_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(c, httpx.ErrValidation)
			return
		}
		filter.SettlementJobID = &id
	}
	if raw := c.Query("account_code"); raw != "" {
		filter.AccountCode = &raw
	}
	if raw := c.Query("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.WriteError(c, httpx.ErrValidation)
			return
		}
		filter.FromTime = &t
	}
	if raw := c.Query("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.WriteError(c, httpx.ErrValidation)
			return
		}
		filter.ToTime = &t
	}

	entries, err := h.svc.ListEntries(c.Request.Context(), filter)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	items := make([]dto.LedgerEntryResponse, 0, len(entries))
	for _, entry := range entries {
		mapped, err := dto.MapLedgerEntry(entry)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		items = append(items, mapped)
	}

	meta := httpx.PageMeta{Limit: page.Limit, HasMore: len(entries) == int(page.Limit)}
	if meta.HasMore && len(entries) > 0 {
		last := entries[len(entries)-1]
		ts, id, err := dto.LedgerCursorFromEntry(last)
		if err == nil {
			cursor := httpx.NextCursor(ts, id)
			meta.NextCursor = &cursor
		}
	}

	c.JSON(http.StatusOK, dto.LedgerEntryListResponse{Items: items, Page: meta})
}

// ListAccounts godoc
// @Summary List ledger accounts
// @Tags ledger
// @Produce json
// @Success 200 {object} dto.LedgerAccountListResponse
// @Router /ledger/accounts [get]
func (h *LedgerHandler) ListAccounts(c *gin.Context) {
	accounts, err := h.svc.ListAccounts(c.Request.Context())
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	items := make([]dto.LedgerAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		mapped, err := dto.MapLedgerAccount(account)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		items = append(items, mapped)
	}
	c.JSON(http.StatusOK, dto.LedgerAccountListResponse{Items: items})
}
