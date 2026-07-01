package retry

import (
	"math/rand"
	"time"
)

// Policy configures exponential backoff between payout retries.
type Policy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// DefaultPolicy returns the standard retry policy from the implementation spec.
func DefaultPolicy() Policy {
	return Policy{
		BaseDelay: 5 * time.Second,
		MaxDelay:  15 * time.Minute,
	}
}

// NextRetryAt computes the earliest time a failed job may be claimed again.
// delay = min(cap, base * 2^retryCount) + jitter(0..1s)
func (p Policy) NextRetryAt(retryCount int32, now time.Time) time.Time {
	if p.BaseDelay <= 0 {
		p.BaseDelay = 5 * time.Second
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 15 * time.Minute
	}

	shift := retryCount
	if shift > 30 {
		shift = 30
	}
	delay := p.BaseDelay * time.Duration(1<<shift)
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	jitter := time.Duration(rand.Int63n(int64(time.Second)))
	return now.Add(delay + jitter)
}
