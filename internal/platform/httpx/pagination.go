package httpx

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/vamshiganesh/arrakin/internal/store"
)

// ErrBadRequest indicates a malformed client request.
var ErrBadRequest = errors.New("bad request")

// ErrValidation indicates request validation failure.
var ErrValidation = errors.New("validation error")

const defaultLimit = 50
const maxLimit = 200

// PageParams holds parsed list query parameters.
type PageParams struct {
	Limit      int32
	CursorTime pgtype.Timestamptz
	CursorID   pgtype.UUID
}

// ParsePageParams reads limit and cursor from query string.
// Cursor format: "<RFC3339Nano>|<uuid>".
func ParsePageParams(c *gin.Context) (PageParams, error) {
	limit := int32(defaultLimit)
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return PageParams{}, errors.Join(ErrValidation, errors.New("limit must be a positive integer"))
		}
		if n > maxLimit {
			n = maxLimit
		}
		limit = int32(n)
	}

	params := PageParams{Limit: limit}
	cursor := c.Query("cursor")
	if cursor == "" {
		return params, nil
	}

	sep := -1
	for i := 0; i < len(cursor); i++ {
		if cursor[i] == '|' {
			sep = i
			break
		}
	}
	if sep <= 0 || sep >= len(cursor)-1 {
		return PageParams{}, errors.Join(ErrValidation, errors.New("cursor must be <timestamp>|<uuid>"))
	}

	ts, err := time.Parse(time.RFC3339Nano, cursor[:sep])
	if err != nil {
		return PageParams{}, errors.Join(ErrValidation, errors.New("invalid cursor timestamp"))
	}
	id, err := uuid.Parse(cursor[sep+1:])
	if err != nil {
		return PageParams{}, errors.Join(ErrValidation, errors.New("invalid cursor id"))
	}

	params.CursorTime = pgtype.Timestamptz{Time: ts, Valid: true}
	params.CursorID = store.UUIDToPgtype(id)
	return params, nil
}

// NextCursor builds a cursor token from a timestamp and id.
func NextCursor(t time.Time, id uuid.UUID) string {
	return t.UTC().Format(time.RFC3339Nano) + "|" + id.String()
}

// PageMeta is pagination metadata returned with list responses.
type PageMeta struct {
	Limit      int32   `json:"limit"`
	NextCursor *string `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}
