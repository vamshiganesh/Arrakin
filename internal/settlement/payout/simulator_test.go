package payout_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/settlement/payout"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

func profilePtr(v string) *string { return &v }

func TestSimulatorSuccess(t *testing.T) {
	sim := payout.NewSimulator()
	jobID := uuid.New()
	result := sim.Execute(payout.ExecuteInput{
		JobID:             jobID,
		AttemptNumber:     1,
		SimulationProfile: profilePtr("success"),
	})
	if result.Err != nil {
		t.Fatalf("expected success, got %v", result.Err)
	}
	if result.PayoutReference != payout.PayoutReference(jobID) {
		t.Fatalf("unexpected payout reference")
	}
}

func TestSimulatorTransientThenSuccess(t *testing.T) {
	sim := payout.NewSimulator()
	jobID := uuid.New()
	profile := "transient_then_success"

	for attempt := int32(1); attempt <= 2; attempt++ {
		result := sim.Execute(payout.ExecuteInput{
			JobID:             jobID,
			AttemptNumber:     attempt,
			SimulationProfile: &profile,
		})
		if result.Err == nil || result.ErrorClass != sqlc.ErrorClassTransient {
			t.Fatalf("attempt %d expected transient failure", attempt)
		}
	}

	result := sim.Execute(payout.ExecuteInput{
		JobID:             jobID,
		AttemptNumber:     3,
		SimulationProfile: &profile,
	})
	if result.Err != nil {
		t.Fatalf("attempt 3 expected success, got %v", result.Err)
	}
}

func TestSimulatorTerminalFailure(t *testing.T) {
	sim := payout.NewSimulator()
	result := sim.Execute(payout.ExecuteInput{
		JobID:             uuid.New(),
		AttemptNumber:     1,
		SimulationProfile: profilePtr("terminal_failure"),
	})
	if result.Err == nil || result.ErrorClass != sqlc.ErrorClassTerminal {
		t.Fatalf("expected terminal failure")
	}
}

func TestPayoutReferenceDeterministic(t *testing.T) {
	id := uuid.MustParse("b2000001-0002-4002-8002-000000000001")
	a := payout.PayoutReference(id)
	b := payout.PayoutReference(id)
	if a != b {
		t.Fatalf("expected deterministic payout reference")
	}
}
