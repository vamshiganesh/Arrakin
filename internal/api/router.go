package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/vamshiganesh/arrakin/internal/api/handlers"
	"github.com/vamshiganesh/arrakin/internal/api/services"
	"github.com/vamshiganesh/arrakin/internal/config"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
	"github.com/vamshiganesh/arrakin/internal/reconciliation"
	"github.com/vamshiganesh/arrakin/internal/scheduler"
	"github.com/vamshiganesh/arrakin/internal/store"

	_ "github.com/vamshiganesh/arrakin/docs"
)

// Dependencies groups infrastructure required by HTTP handlers.
type Dependencies struct {
	Logger      *slog.Logger
	Config      config.Config
	DB          handlers.DBPinger
	Redis       handlers.RedisPinger
	Store       *store.Store
	Scheduler   *scheduler.Scheduler
	Idempotency *idempotency.Service
}

// NewRouter builds the Gin engine with middleware and routes.
func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpx.RequestID())
	router.Use(httpx.RequestLogger(deps.Logger))

	health := handlers.NewHealthHandler(deps.DB, deps.Redis)
	router.GET("/healthz", health.Live)
	router.GET("/readyz", health.Ready)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	reconSvc := reconciliation.New(deps.Store.Repos().Reconciliation)
	settlementSvc := services.NewSettlementService(deps.Store, nil)
	ledgerSvc := services.NewLedgerService(deps.Store)
	reconciliationAPI := services.NewReconciliationService(deps.Store, reconSvc)
	auditSvc := services.NewAuditService(deps.Store)
	adminSvc := services.NewAdminService(deps.Scheduler)

	settlementHandler := handlers.NewSettlementHandler(settlementSvc, deps.Store, deps.Idempotency)
	ledgerHandler := handlers.NewLedgerHandler(ledgerSvc)
	reconciliationHandler := handlers.NewReconciliationHandler(reconciliationAPI, deps.Store, deps.Idempotency)
	auditHandler := handlers.NewAuditHandler(auditSvc)
	adminHandler := handlers.NewAdminHandler(adminSvc, deps.Store, deps.Idempotency)

	apiKey := httpx.APIKey(httpx.APIKeyConfig{
		ExpectedKey: deps.Config.APIKey,
		DevBypass:   deps.Config.IsDevelopment(),
	})

	v1 := router.Group("/api/v1")
	{
		jobs := v1.Group("/settlement-jobs")
		{
			jobs.GET("", settlementHandler.ListJobs)
			jobs.GET("/:id", settlementHandler.GetJob)
			jobs.GET("/:id/attempts", settlementHandler.ListAttempts)
			jobs.POST("/:id/replay", apiKey, settlementHandler.ReplayJob)
			jobs.POST("/:id/requeue", apiKey, settlementHandler.RequeueJob)
		}

		ledger := v1.Group("/ledger")
		{
			ledger.GET("/entries", ledgerHandler.ListEntries)
			ledger.GET("/accounts", ledgerHandler.ListAccounts)
		}

		recon := v1.Group("/reconciliation")
		{
			recon.GET("/latest", reconciliationHandler.GetLatest)
			recon.GET("/snapshots", reconciliationHandler.ListSnapshots)
			recon.POST("/run", apiKey, reconciliationHandler.Run)
		}

		v1.GET("/audit/events", auditHandler.ListEvents)

		admin := v1.Group("/admin", apiKey)
		{
			admin.POST("/scheduler/tick", adminHandler.SchedulerTick)
		}
	}

	return router
}
