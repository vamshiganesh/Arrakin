package services

import (
	"context"

	"github.com/vamshiganesh/arrakin/internal/scheduler"
)

// AdminService exposes operational admin actions.
type AdminService struct {
	scheduler *scheduler.Scheduler
}

// NewAdminService creates an admin API service.
func NewAdminService(sched *scheduler.Scheduler) *AdminService {
	return &AdminService{scheduler: sched}
}

// TriggerSchedulerScan runs one maturity enqueue cycle.
func (s *AdminService) TriggerSchedulerScan(ctx context.Context) (int, error) {
	return s.scheduler.TickOnce(ctx)
}
