package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505"}
	if !isUniqueViolation(pgErr) {
		t.Fatal("expected unique violation")
	}
	if isUniqueViolation(errors.New("other")) {
		t.Fatal("expected false for generic error")
	}
}

func TestStringPtr(t *testing.T) {
	ptr := StringPtr("worker-1")
	if ptr == nil || *ptr != "worker-1" {
		t.Fatalf("unexpected pointer value: %v", ptr)
	}
}
