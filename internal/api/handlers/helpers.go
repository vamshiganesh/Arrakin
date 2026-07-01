package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// runIdempotent executes fn with idempotency key deduplication when Idempotency-Key is present.
// When the header is absent, fn runs once without persistence.
func runIdempotent(
	c *gin.Context,
	st *store.Store,
	svc *idempotency.Service,
	scope string,
	fn func(ctx context.Context) (int, any, error),
) {
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		status, body, err := fn(c.Request.Context())
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(status, body)
		return
	}

	var stored idempotency.StoredResponse
	var replayed bool
	err := st.WithTx(c.Request.Context(), func(ctx context.Context, q *sqlc.Queries) error {
		var execErr error
		stored, replayed, execErr = svc.Execute(ctx, q, scope, key, "", func(ctx context.Context) (idempotency.StoredResponse, error) {
			status, body, err := fn(ctx)
			if err != nil {
				return idempotency.StoredResponse{}, err
			}
			raw, err := json.Marshal(body)
			if err != nil {
				return idempotency.StoredResponse{}, err
			}
			return idempotency.StoredResponse{StatusCode: status, Body: raw}, nil
		})
		return execErr
	})
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	if replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	c.Data(stored.StatusCode, "application/json", stored.Body)
}

func parseUUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	raw := c.Param(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, httpx.ErrValidation
	}
	return id, nil
}

func actorID(c *gin.Context) string {
	if key := c.GetHeader("X-API-Key"); key != "" {
		return "api-key:" + key[:min(8, len(key))]
	}
	return "admin"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
