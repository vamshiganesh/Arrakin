//go:build integration

package ledger_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/domain/money"
	"github.com/vamshiganesh/arrakin/internal/domain/settlement"
	"github.com/vamshiganesh/arrakin/internal/ledger"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

func testPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://arrakin:arrakin@localhost:5432/arrakin?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func seedInvestor(t *testing.T, pool *pgxpool.Pool, ctx context.Context, investorID uuid.UUID) {
	t.Helper()
	suffix := investorID.String()[:8]
	if _, err := pool.Exec(ctx, `
		INSERT INTO investors (id, external_ref, display_name)
		VALUES ($1, $2, $3)
	`, investorID, "integ-"+suffix, "Integration Investor "+suffix); err != nil {
		t.Fatalf("seed investor: %v", err)
	}
}

func TestPostSettlementWritesBalancedEntries(t *testing.T) {
	pool, ctx := testPool(t)
	s := store.New(pool)
	posting := ledger.NewPostingService(s.Repos().Ledger)

	jobID := uuid.New()
	investorID := uuid.New()
	investmentID := uuid.New()
	maturityID := uuid.New()
	seedInvestor(t, pool, ctx, investorID)
	breakdown := settlement.Breakdown{
		PrincipalCents:      money.Cents(1_000_000),
		GrossReturnCents:    money.Cents(80_000),
		PlatformFeeCents:    money.Cents(800),
		WithholdingTaxCents: money.Cents(11_880),
		NetPayoutCents:      money.Cents(1_067_320),
		Currency:            "USD",
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO investments (id, investor_id, principal_cents, annual_rate_bps, term_days, currency)
		VALUES ($1, $2, 1000000, 800, 365, 'USD')
	`, investmentID, investorID); err != nil {
		t.Fatalf("seed investment: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO maturity_schedules (id, investment_id, matures_at, status)
		VALUES ($1, $2, now() - interval '1 hour', 'pending')
	`, maturityID, investmentID); err != nil {
		t.Fatalf("seed maturity: %v", err)
	}

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		job, _, err := s.Repos().SettlementJobs.CreateIdempotent(ctx, q, store.CreateJobParams{
			MaturityScheduleID:  store.UUIDToPgtype(maturityID),
			InvestmentID:        store.UUIDToPgtype(investmentID),
			IdempotencyKey:      "ledger-test-" + jobID.String(),
			PrincipalCents:      breakdown.PrincipalCents.Int64(),
			GrossReturnCents:    breakdown.GrossReturnCents.Int64(),
			PlatformFeeCents:    breakdown.PlatformFeeCents.Int64(),
			WithholdingTaxCents: breakdown.WithholdingTaxCents.Int64(),
			NetPayoutCents:      breakdown.NetPayoutCents.Int64(),
			MaxRetries:          3,
		})
		if err != nil {
			t.Fatalf("create job: %v", err)
		}
		jobUUID, err := store.PgtypeToUUID(job.ID)
		if err != nil {
			t.Fatal(err)
		}

		entries, err := posting.PostSettlement(ctx, q, ledger.PostSettlementInput{
			JobID:      jobUUID,
			InvestorID: investorID,
			Breakdown:  breakdown,
		})
		if err != nil {
			t.Fatalf("post settlement: %v", err)
		}
		if len(entries) != 4 {
			t.Fatalf("expected 4 ledger lines, got %d", len(entries))
		}

		again, err := posting.PostSettlement(ctx, q, ledger.PostSettlementInput{
			JobID:      jobUUID,
			InvestorID: investorID,
			Breakdown:  breakdown,
		})
		if err != nil {
			t.Fatalf("idempotent post: %v", err)
		}
		if len(again) != 4 {
			t.Fatalf("expected 4 lines on replay, got %d", len(again))
		}

		return errors.New("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback, got %v", err)
	}
}
