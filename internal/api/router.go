package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/vamshiganesh/arrakin/internal/api/handlers"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
)

// Dependencies groups infrastructure required by HTTP handlers.
type Dependencies struct {
	Logger *slog.Logger
	DB     handlers.DBPinger
	Redis  handlers.RedisPinger
}

// DBPinger is re-exported for callers wiring the router.
type DBPinger = handlers.DBPinger

// RedisPinger is re-exported for callers wiring the router.
type RedisPinger = handlers.RedisPinger

// NewRouter builds the Gin engine with middleware and routes.
func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpx.RequestID())
	router.Use(httpx.RequestLogger(deps.Logger))

	health := handlers.NewHealthHandler(deps.DB, deps.Redis)
	router.GET("/healthz", health.Live)
	router.GET("/readyz", health.Ready)

	return router
}
