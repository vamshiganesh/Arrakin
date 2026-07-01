package metrics

import (
	"sync/atomic"
)

// SettlementMetrics tracks coarse processing counters for observability.
type SettlementMetrics struct {
	JobsEnqueued     atomic.Int64
	JobsClaimed      atomic.Int64
	JobsSucceeded    atomic.Int64
	JobsRetried      atomic.Int64
	JobsDeadLettered atomic.Int64
	SchedulerTicks   atomic.Int64
	LeasesExpired    atomic.Int64
}

// Global is the process-wide metrics registry.
var Global SettlementMetrics
