package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/platform/metrics"
	"github.com/vamshiganesh/arrakin/internal/settlement/orchestrator"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// Config controls scheduler timing and lease recovery.
type Config struct {
	Interval        time.Duration
	ReaperInterval  time.Duration
	JobLeaseTimeout time.Duration
}

// Scheduler periodically enqueues due maturities and reaps stale worker leases.
type Scheduler struct {
	cfg          Config
	orchestrator *orchestrator.Service
	store        *store.Store
	repos        store.Repositories
	audit        *audit.Publisher
}

// New creates a maturity scheduler.
func New(
	cfg Config,
	orch *orchestrator.Service,
	st *store.Store,
	auditPub *audit.Publisher,
) *Scheduler {
	return &Scheduler{
		cfg:          cfg,
		orchestrator: orch,
		store:        st,
		repos:        st.Repos(),
		audit:        auditPub,
	}
}

// Run starts the scheduler loop until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	interval := s.cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	reaperInterval := s.cfg.ReaperInterval
	if reaperInterval <= 0 {
		reaperInterval = 60 * time.Second
	}

	tick := time.NewTicker(interval)
	reaper := time.NewTicker(reaperInterval)
	defer tick.Stop()
	defer reaper.Stop()

	slog.Info("scheduler started", "interval", interval, "reaper_interval", reaperInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case <-tick.C:
			s.runTick(ctx)
		case <-reaper.C:
			s.reapStaleLeases(ctx)
		}
	}
}

// TickOnce runs a single scheduler scan (useful for tests and admin triggers).
func (s *Scheduler) TickOnce(ctx context.Context) (int, error) {
	return s.orchestrator.EnqueueDueMaturities(ctx)
}

func (s *Scheduler) runTick(ctx context.Context) {
	start := time.Now()
	metrics.Global.SchedulerTicks.Add(1)

	created, err := s.orchestrator.EnqueueDueMaturities(ctx)
	if err != nil {
		slog.Error("scheduler tick failed", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	slog.Info("scheduler tick complete",
		"jobs_created", created,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

func (s *Scheduler) reapStaleLeases(ctx context.Context) {
	cutoff := time.Now().Add(-s.cfg.JobLeaseTimeout)
	err := s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		jobs, err := s.repos.SettlementJobs.ExpireStaleLeases(ctx, q, cutoff)
		if err != nil {
			return err
		}
		for _, job := range jobs {
			jobID, err := store.PgtypeToUUID(job.ID)
			if err != nil {
				return err
			}
			if _, err := s.audit.PublishJobTransition(ctx, q, jobID, audit.ActionSettlementJobLeaseExpired, jobID.String(), map[string]any{
				"processing_owner": job.ProcessingOwner,
			}); err != nil {
				return err
			}
			slog.Warn("expired stale processing lease", "job_id", jobID, "worker", job.ProcessingOwner)
		}
		if len(jobs) > 0 {
			metrics.Global.LeasesExpired.Add(int64(len(jobs)))
		}
		return nil
	})
	if err != nil {
		slog.Error("lease reaper failed", "error", err)
	}
}
