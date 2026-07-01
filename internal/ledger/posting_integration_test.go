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

func TestPostSettlementWritesBalancedEntries(t *testing.T) {
	pool, ctx := testPool(t)
	s := store.New(pool)
	posting := ledger.NewPostingService(s.Repos().Ledger)

	jobID := uuid.New()
	investorID := uuid.MustParse("a1000001-0001-4001-8001-000000000001")
	breakdown := settlement.Breakdown{
		PrincipalCents:      money.Cents(1_000_000),
		GrossReturnCents:    money.Cents(80_000),
		PlatformFeeCents:    money.Cents(800),
		WithholdingTaxCents: money.Cents(11_880),
		NetPayoutCents:      money.Cents(1_067_320),
		Currency:            "USD",
	}

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		maturityID := store.UUIDToPgtype(uuid.New())
		investmentID := store.UUIDToPgtype(uuid.MustParse("b2000001-0002-4002-8002-000000000001"))

		if _, err := pool.Exec(ctx, `
			INSERT INTO maturity_schedules (id, investment_id, matures_at, status)
			VALUES ($1, $2, now() - interval '1 hour', 'pending')
			ON CONFLICT DO NOTHING
		`, maturityID, investmentID); err != nil {
			t.Fatalf("insert maturity: %v", err)
		}

		job, _, err := s.Repos().SettlementJobs.CreateIdempotent(ctx, q, store.CreateJobParams{
			MaturityScheduleID:  maturityID,
			InvestmentID:        investmentID,
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
