package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vamshiganesh/arrakin/internal/api/handlers"
)

func TestHealthLive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := handlers.NewHealthHandler(stubPinger{}, stubPinger{})

	router := gin.New()
	router.GET("/healthz", h.Live)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

type stubPinger struct{}

func (stubPinger) Ping(context.Context) error { return nil }
