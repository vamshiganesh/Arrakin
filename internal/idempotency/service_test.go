package idempotency_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
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

func TestExecuteStoresAndReplaysResponse(t *testing.T) {
	pool, ctx := testPool(t)
	s := store.New(pool)
	svc := idempotency.NewService(s.Repos().Idempotency, time.Hour)

	scope := "test.execute"
	key := "key-" + time.Now().Format("150405.000000000")

	var calls int
	handler := func(ctx context.Context) (idempotency.StoredResponse, error) {
		calls++
		body, _ := json.Marshal(map[string]string{"status": "ok"})
		return idempotency.StoredResponse{StatusCode: 201, Body: body}, nil
	}

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		first, replay, err := svc.Execute(ctx, q, scope, key, "hash-1", handler)
		if err != nil {
			t.Fatalf("first execute: %v", err)
		}
		if replay {
			t.Fatal("expected first call not to replay")
		}
		if first.StatusCode != 201 {
			t.Fatalf("unexpected status: %d", first.StatusCode)
		}

		second, replay, err := svc.Execute(ctx, q, scope, key, "hash-1", handler)
		if err != nil {
			t.Fatalf("second execute: %v", err)
		}
		if !replay {
			t.Fatal("expected replay on second call")
		}
		if string(second.Body) != string(first.Body) {
			t.Fatalf("body mismatch on replay")
		}
		return errors.New("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("handler should run once, got %d", calls)
	}
}

func TestReserveConflictWithoutCompletedResponse(t *testing.T) {
	pool, ctx := testPool(t)
	s := store.New(pool)
	svc := idempotency.NewService(s.Repos().Idempotency, time.Hour)

	scope := "test.reserve"
	key := "key-" + time.Now().Format("150405.000000001")

	err := s.WithTx(ctx, func(ctx context.Context, q *sqlc.Queries) error {
		if err := svc.Reserve(ctx, q, scope, key, "hash"); err != nil {
			t.Fatalf("reserve: %v", err)
		}
		_, _, err := svc.Execute(ctx, q, scope, key, "hash", func(ctx context.Context) (idempotency.StoredResponse, error) {
			return idempotency.StoredResponse{StatusCode: 200, Body: []byte(`{}`)}, nil
		})
		if !errors.Is(err, idempotency.ErrKeyInProgress) {
			t.Fatalf("expected ErrKeyInProgress, got %v", err)
		}
		return errors.New("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback, got %v", err)
	}
}
