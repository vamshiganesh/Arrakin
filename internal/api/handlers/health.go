package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DBPinger checks Postgres connectivity.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// RedisPinger checks Redis connectivity.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	db    DBPinger
	redis RedisPinger
}

// NewHealthHandler constructs health endpoints backed by infrastructure checks.
func NewHealthHandler(db DBPinger, redis RedisPinger) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

type healthResponse struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Live returns 200 when the process is running.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

// Ready returns 200 only when dependencies are reachable.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{
		"postgres": "ok",
		"redis":    "ok",
	}

	if err := h.db.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
	}
	if err := h.redis.Ping(ctx); err != nil {
		checks["redis"] = err.Error()
	}

	for _, status := range checks {
		if status != "ok" {
			c.JSON(http.StatusServiceUnavailable, readinessResponse{
				Status: "unavailable",
				Checks: checks,
			})
			return
		}
	}

	c.JSON(http.StatusOK, readinessResponse{
		Status: "ok",
		Checks: checks,
	})
}
