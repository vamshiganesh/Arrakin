package retry_test

import (
	"testing"
	"time"

	"github.com/vamshiganesh/arrakin/internal/settlement/retry"
)

func TestNextRetryAtExponentialGrowth(t *testing.T) {
	policy := retry.Policy{BaseDelay: 5 * time.Second, MaxDelay: 15 * time.Minute}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	first := policy.NextRetryAt(0, now)
	second := policy.NextRetryAt(1, now)

	if !first.After(now) {
		t.Fatalf("expected first retry in the future")
	}
	if !second.After(first) {
		t.Fatalf("expected second retry after first: first=%s second=%s", first, second)
	}
}

func TestNextRetryAtCapsAtMaxDelay(t *testing.T) {
	policy := retry.Policy{BaseDelay: time.Minute, MaxDelay: 2 * time.Minute}
	now := time.Now()
	got := policy.NextRetryAt(20, now)
	maxAllowed := now.Add(2*time.Minute + time.Second)
	if got.After(maxAllowed) {
		t.Fatalf("retry delay exceeded cap: %s", got)
	}
}
