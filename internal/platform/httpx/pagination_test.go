package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
)

func TestParsePageParamsDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/items", nil)

	page, err := httpx.ParsePageParams(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Limit != 50 {
		t.Fatalf("expected default limit 50, got %d", page.Limit)
	}
}

func TestParsePageParamsCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ts := time.Now().UTC()
	id := uuid.New()
	cursor := httpx.NextCursor(ts, id)
	c.Request = httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor, nil)

	page, err := httpx.ParsePageParams(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !page.CursorTime.Valid {
		t.Fatal("expected cursor time")
	}
	if !page.CursorID.Valid {
		t.Fatal("expected cursor id")
	}
}

func TestNextCursorRoundTrip(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	id := uuid.New()
	token := httpx.NextCursor(ts, id)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/items?cursor="+token, nil)
	page, err := httpx.ParsePageParams(c)
	if err != nil {
		t.Fatalf("parse cursor: %v", err)
	}
	gotID, err := uuid.FromBytes(page.CursorID.Bytes[:])
	if err != nil {
		t.Fatalf("cursor id: %v", err)
	}
	if gotID != id {
		t.Fatalf("cursor id mismatch")
	}
}
