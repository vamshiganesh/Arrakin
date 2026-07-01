package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vamshiganesh/arrakin/internal/store"
)

// Config controls worker pool concurrency and polling behavior.
type Config struct {
	WorkerCount  int
	PollInterval time.Duration
	BatchSize    int
}

// Pool runs concurrent workers that claim and process settlement jobs.
type Pool struct {
	cfg       Config
	processor *Processor
	instance  string
	wg        sync.WaitGroup
}

// NewPool creates a worker pool.
func NewPool(cfg Config, processor *Processor, instanceID string) *Pool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1
	}
	if instanceID == "" {
		instanceID = "arrakin"
	}
	return &Pool{
		cfg:       cfg,
		processor: processor,
		instance:  instanceID,
	}
}

// Run starts worker goroutines until ctx is cancelled.
func (p *Pool) Run(ctx context.Context) {
	slog.Info("worker pool started",
		"workers", p.cfg.WorkerCount,
		"poll_interval", p.cfg.PollInterval,
		"batch_size", p.cfg.BatchSize,
		"instance", p.instance,
	)

	for i := 0; i < p.cfg.WorkerCount; i++ {
		p.wg.Add(1)
		workerID := fmt.Sprintf("%s-worker-%d", p.instance, i)
		go func(id string) {
			defer p.wg.Done()
			p.loop(ctx, id)
		}(workerID)
	}

	<-ctx.Done()
	p.wg.Wait()
	slog.Info("worker pool stopped")
}

func (p *Pool) loop(ctx context.Context, workerID string) {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.drain(ctx, workerID)
		}
	}
}

func (p *Pool) drain(ctx context.Context, workerID string) {
	for i := 0; i < p.cfg.BatchSize; i++ {
		if err := p.processor.ProcessOne(ctx, workerID); err != nil {
			if errors.Is(err, store.ErrNoJobAvailable) {
				return
			}
			return
		}
	}
}
