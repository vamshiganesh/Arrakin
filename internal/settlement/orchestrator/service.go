package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vamshiganesh/arrakin/internal/audit"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
	"github.com/vamshiganesh/arrakin/internal/platform/metrics"
	"github.com/vamshiganesh/arrakin/internal/settlement/calculator"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// Service coordinates maturity discovery and idempotent settlement job creation.
type Service struct {
	store    *store.Store
	repos    store.Repositories
	calc     *calculator.Service
	audit    *audit.Publisher
	maxRetry int32
}

// New creates an orchestrator service.
func New(
	st *store.Store,
	calc *calculator.Service,
	auditPub *audit.Publisher,
	maxRetries int,
) *Service {
	return &Service{
		store:    st,
		repos:    st.Repos(),
		calc:     calc,
		audit:    auditPub,
		maxRetry: int32(maxRetries),
	}
}

// EnqueueDueMaturities scans due maturities and creates settlement jobs inside one transaction.
func (s *Service) EnqueueDueMaturities(ctx context.Context) (int, error) {
	created := 0
	err := s.store.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		due, err := s.repos.Maturities.ListDue(ctx, q)
		if err != nil {
			return fmt.Errorf("list due maturities: %w", err)
		}

		for _, row := range due {
			terms := settlement.InvestmentTerms{
				PrincipalCents: row.Investment.PrincipalCents,
				AnnualRateBPS:  int(row.Investment.AnnualRateBps),
				TermDays:       int(row.Investment.TermDays),
				Currency:       row.Investment.Currency,
			}
			breakdown, err := s.calc.Calculate(terms)
			if err != nil {
				return fmt.Errorf("calculate settlement for maturity %s: %w", row.MaturitySchedule.ID.Bytes, err)
			}

			maturityID, err := store.PgtypeToUUID(row.MaturitySchedule.ID)
			if err != nil {
				return err
			}

			job, inserted, err := s.repos.SettlementJobs.CreateIdempotent(ctx, q, store.CreateJobParams{
				MaturityScheduleID:  row.MaturitySchedule.ID,
				InvestmentID:        row.Investment.ID,
				IdempotencyKey:      fmt.Sprintf("maturity:%s", maturityID),
				PrincipalCents:      breakdown.PrincipalCents.Int64(),
				GrossReturnCents:    breakdown.GrossReturnCents.Int64(),
				PlatformFeeCents:    breakdown.PlatformFeeCents.Int64(),
				WithholdingTaxCents: breakdown.WithholdingTaxCents.Int64(),
				NetPayoutCents:      breakdown.NetPayoutCents.Int64(),
				MaxRetries:          s.maxRetry,
			})
			if err != nil {
				return fmt.Errorf("create settlement job: %w", err)
			}
			if !inserted {
				continue
			}

			created++
			jobID, err := store.PgtypeToUUID(job.ID)
			if err != nil {
				return err
			}

			if _, err := s.audit.PublishJobTransition(ctx, q, jobID, audit.ActionSettlementJobCreated, maturityID.String(), map[string]any{
				"maturity_schedule_id": maturityID.String(),
				"net_payout_cents":     breakdown.NetPayoutCents.Int64(),
			}); err != nil {
				return fmt.Errorf("audit job created: %w", err)
			}

			slog.Info("settlement job enqueued",
				"job_id", jobID,
				"maturity_schedule_id", maturityID,
				"net_payout_cents", breakdown.NetPayoutCents.Int64(),
			)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if created > 0 {
		metrics.Global.JobsEnqueued.Add(int64(created))
	}
	return created, nil
}
