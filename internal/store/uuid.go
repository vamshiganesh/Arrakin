package store

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

// UUIDToPgtype converts a UUID to pgtype.UUID.
func UUIDToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// PgtypeToUUID converts pgtype.UUID to uuid.UUID.
func PgtypeToUUID(id pgtype.UUID) (uuid.UUID, error) {
	if !id.Valid {
		return uuid.Nil, fmt.Errorf("invalid uuid")
	}
	return uuid.FromBytes(id.Bytes[:])
}

// StringToPgtypeUUID parses a string into pgtype.UUID.
func StringToPgtypeUUID(value string) (pgtype.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return UUIDToPgtype(id), nil
}

// StringPtr returns a pointer to s.
func StringPtr(s string) *string {
	return &s
}

// TextPtr converts optional text to *string for sqlc nullable params.
func TextPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

// Int32Ptr returns a pointer to n.
func Int32Ptr(n int32) *int32 {
	return &n
}

// ErrorClassPtr returns a pointer to an error class enum value.
func ErrorClassPtr(class sqlc.ErrorClass) *sqlc.ErrorClass {
	return &class
}
