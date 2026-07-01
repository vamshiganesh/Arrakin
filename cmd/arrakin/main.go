// @title Arrakin API
// @version 1.0
// @description Settlement, ledger, and payout engine HTTP API
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/vamshiganesh/arrakin/internal/api"
	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/config"
	"github.com/vamshiganesh/arrakin/internal/ledger"
	"github.com/vamshiganesh/arrakin/internal/platform/db"
	"github.com/vamshiganesh/arrakin/internal/platform/logging"
	"github.com/vamshiganesh/arrakin/internal/platform/redis"
	"github.com/vamshiganesh/arrakin/internal/scheduler"
	"github.com/vamshiganesh/arrakin/internal/settlement/calculator"
	"github.com/vamshiganesh/arrakin/internal/settlement/orchestrator"
	"github.com/vamshiganesh/arrakin/internal/settlement/payout"
	"github.com/vamshiganesh/arrakin/internal/settlement/retry"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	if !cfg.IsDevelopment() {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	redisClient, err := redis.New(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
	}()

	st := store.New(pool.Pool)
	repos := st.Repos()

	calc, err := calculator.New(calculator.Config{
		PlatformFeeBPS:    cfg.PlatformFeeBPS,
		WithholdingTaxBPS: cfg.WithholdingTaxBPS,
	})
	if err != nil {
		return fmt.Errorf("settlement calculator: %w", err)
	}

	auditPub := audit.NewPublisher(repos.Audit)
	idemSvc := idempotency.NewService(repos.Idempotency, 24*time.Hour)
	ledgerSvc := ledger.NewPostingService(repos.Ledger)
	payoutGateway := payout.NewSimulator()
	retryPolicy := retry.Policy{BaseDelay: cfg.RetryBaseDelay, MaxDelay: cfg.RetryMaxDelay}

	orch := orchestrator.New(st, calc, auditPub, cfg.MaxRetries)
	sched := scheduler.New(scheduler.Config{
		Interval:        cfg.SchedulerInterval,
		ReaperInterval:  cfg.ReaperInterval,
		JobLeaseTimeout: cfg.JobLeaseTimeout,
	}, orch, st, auditPub)
	processor := worker.NewProcessor(st, ledgerSvc, auditPub, payoutGateway, retryPolicy)
	workerPool := worker.NewPool(worker.Config{
		WorkerCount:  cfg.WorkerCount,
		PollInterval: cfg.WorkerPollInterval,
		BatchSize:    cfg.WorkerBatchSize,
	}, processor, "arrakin")

	engineCtx, engineCancel := context.WithCancel(ctx)
	defer engineCancel()
	go sched.Run(engineCtx)
	go workerPool.Run(engineCtx)

	router := api.NewRouter(api.Dependencies{
		Logger:      logger,
		Config:      cfg,
		DB:          pool,
		Redis:       redisClient,
		Store:       st,
		Scheduler:   sched,
		Audit:       auditPub,
		Idempotency: idemSvc,
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting http server",
			"addr", server.Addr,
			"app_env", cfg.AppEnv,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	engineCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}
