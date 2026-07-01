package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

func testStore(t *testing.T) (*store.Store, context.Context) {
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

	return store.New(pool), ctx
}

func demoMaturityID() uuid.UUID {
	return uuid.MustParse("c3000001-0003-4003-8003-000000000004")
}

func demoInvestmentID() uuid.UUID {
	return uuid.MustParse("b2000001-0002-4002-8002-000000000004")
}

func demoMaturityIDForClaim() uuid.UUID {
	return uuid.MustParse("c3000001-0003-4003-8003-000000000005")
}

func demoInvestmentIDForClaim() uuid.UUID {
	return uuid.MustParse("b2000001-0002-4002-8002-000000000005")
}

func TestCreateSettlementJobIdempotent(t *testing.T) {
	s, ctx := testStore(t)
	repos := s.Repos()
	pool := s.Pool()

	investmentID := uuid.New()
	maturityID := uuid.New()
	investorID := uuid.MustParse("a1000001-0001-4001-8001-000000000001")
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
		params := store.CreateJobParams{
			MaturityScheduleID:  store.UUIDToPgtype(maturityID),
			InvestmentID:        store.UUIDToPgtype(investmentID),
			IdempotencyKey:      "enqueue:" + maturityID.String(),
			PrincipalCents:      1_000_000,
			GrossReturnCents:    80_000,
			PlatformFeeCents:    800,
			WithholdingTaxCents: 11_880,
			NetPayoutCents:    1_067_320,
			MaxRetries:          5,
		}

		first, created, err := repos.SettlementJobs.CreateIdempotent(ctx, q, params)
		if err != nil {
			t.Fatalf("first create: %v", err)
		}
		if !created {
			t.Fatal("expected first create to insert row")
		}

		second, created, err := repos.SettlementJobs.CreateIdempotent(ctx, q, params)
		if err != nil {
			t.Fatalf("second create: %v", err)
		}
		if created {
			t.Fatal("expected second create to be idempotent no-op")
		}
		if first.ID != second.ID {
			t.Fatalf("expected same job id, got %s vs %s", first.ID.Bytes, second.ID.Bytes)
		}

		return errors.New("rollback test tx")
	})
	if err == nil || err.Error() != "rollback test tx" {
		t.Fatalf("expected rollback sentinel, got %v", err)
	}
}

func TestClaimSettlementJob(t *testing.T) {
	s, ctx := testStore(t)
	repos := s.Repos()
	pool := s.Pool()

	investmentID := uuid.New()
	maturityID := uuid.New()
	investorID := uuid.MustParse("a1000001-0001-4001-8001-000000000001")
	if _, err := pool.Exec(ctx, `
		INSERT INTO investments (id, investor_id, principal_cents, annual_rate_bps, term_days, currency)
		VALUES ($1, $2, 100000, 800, 365, 'USD')
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
		_, created, err := repos.SettlementJobs.CreateIdempotent(ctx, q, store.CreateJobParams{
			MaturityScheduleID:  store.UUIDToPgtype(maturityID),
			InvestmentID:        store.UUIDToPgtype(investmentID),
			IdempotencyKey:      "test-claim-" + uuid.NewString(),
			PrincipalCents:     100_00,
			GrossReturnCents:   10_00,
			PlatformFeeCents:   1_00,
			WithholdingTaxCents: 1_00,
			NetPayoutCents:     108_00,
			MaxRetries:         3,
		})
		if err != nil || !created {
			t.Fatalf("create job: created=%v err=%v", created, err)
		}

		job, err := repos.SettlementJobs.Claim(ctx, q, "worker-test-1")
		if err != nil {
			t.Fatalf("claim job: %v", err)
		}
		if job.Status != sqlc.SettlementJobStatusProcessing {
			t.Fatalf("expected processing, got %s", job.Status)
		}

		_, err = repos.SettlementJobs.Claim(ctx, q, "worker-test-2")
		if !errors.Is(err, store.ErrNoJobAvailable) {
			t.Fatalf("expected ErrNoJobAvailable, got %v", err)
		}

		return errors.New("rollback test tx")
	})
	if err == nil || err.Error() != "rollback test tx" {
		t.Fatalf("expected rollback sentinel, got %v", err)
	}
}

func TestLedgerPostEntriesImmutable(t *testing.T) {
	s, ctx := testStore(t)
	repos := s.Repos()

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		jobID := store.UUIDToPgtype(uuid.New())
		groupID := store.UUIDToPgtype(uuid.New())

		lines := []store.LedgerLineInput{
			{
				AccountCode: "INVESTOR_PAYABLE:test",
				AccountName: "Investor Payable",
				AccountType: "liability",
				Side:        "D",
				AmountCents: 100_00,
				Currency:    "USD",
				Description: "debit payable",
				Metadata:    []byte(`{}`),
			},
			{
				AccountCode: "INVESTOR_WALLET:test",
				AccountName: "Investor Wallet",
				AccountType: "asset",
				Side:        "C",
				AmountCents: 100_00,
				Currency:    "USD",
				Description: "credit wallet",
				Metadata:    []byte(`{}`),
			},
		}

		if _, err := repos.Ledger.PostEntries(ctx, q, jobID, groupID, lines); err == nil {
			t.Fatal("expected foreign key error without settlement job")
		}

		return errors.New("rollback test tx")
	})
	if err == nil || err.Error() != "rollback test tx" {
		t.Fatalf("expected rollback sentinel, got %v", err)
	}
}

func TestReconciliationSnapshot(t *testing.T) {
	s, ctx := testStore(t)
	repos := s.Repos()

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		snapshot, err := repos.Reconciliation.CreateSnapshot(ctx, q)
		if err != nil {
			t.Fatalf("create snapshot: %v", err)
		}
		if snapshot.SnapshotAt.Time.IsZero() {
			t.Fatal("expected snapshot timestamp")
		}
		return errors.New("rollback test tx")
	})
	if err == nil || err.Error() != "rollback test tx" {
		t.Fatalf("expected rollback sentinel, got %v", err)
	}
}

func TestIdempotencyReserveConflict(t *testing.T) {
	s, ctx := testStore(t)
	repos := s.Repos()

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		key := "idem-" + uuid.NewString()
		params := store.ReserveIdempotencyParams{
			Scope:     "test.scope",
			Key:       key,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		if _, err := repos.Idempotency.Reserve(ctx, q, params); err != nil {
			t.Fatalf("first reserve: %v", err)
		}
		if _, err := repos.Idempotency.Reserve(ctx, q, params); !errors.Is(err, store.ErrIdempotencyKeyExists) {
			t.Fatalf("expected ErrIdempotencyKeyExists, got %v", err)
		}
		return errors.New("rollback test tx")
	})
	if err == nil || err.Error() != "rollback test tx" {
		t.Fatalf("expected rollback sentinel, got %v", err)
	}
}
