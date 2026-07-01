package payout

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// ExecuteInput is passed to the payout gateway for one attempt.
type ExecuteInput struct {
	JobID             uuid.UUID
	AttemptNumber     int32
	SimulationProfile *string
	NetPayoutCents    int64
}

// Result is the outcome of a payout attempt.
type Result struct {
	PayoutReference string
	ErrorClass      sqlc.ErrorClass
	Err             error
}

// Gateway executes investor payouts.
type Gateway interface {
	Execute(in ExecuteInput) Result
}

// Simulator is a demo payout rail driven by investment simulation_profile.
type Simulator struct{}

// NewSimulator creates a payout simulator for local/demo environments.
func NewSimulator() *Simulator {
	return &Simulator{}
}

// Execute simulates payout success, transient failure, or terminal failure.
func (s *Simulator) Execute(in ExecuteInput) Result {
	ref := PayoutReference(in.JobID)

	profile := ""
	if in.SimulationProfile != nil {
		profile = *in.SimulationProfile
	}

	switch profile {
	case "", "success":
		return Result{PayoutReference: ref}
	case "terminal_failure":
		return Result{
			ErrorClass: sqlc.ErrorClassTerminal,
			Err:        fmt.Errorf("simulated terminal payout failure"),
		}
	case "transient_then_success":
		// Fail the first two attempts, succeed on the third.
		if in.AttemptNumber < 3 {
			return Result{
				ErrorClass: sqlc.ErrorClassTransient,
				Err:        fmt.Errorf("simulated transient payout failure (attempt %d)", in.AttemptNumber),
			}
		}
		return Result{PayoutReference: ref}
	default:
		return Result{PayoutReference: ref}
	}
}

// PayoutReference returns a deterministic payout reference for idempotent replays.
func PayoutReference(jobID uuid.UUID) string {
	return fmt.Sprintf("pay_sim_%s", jobID)
}
