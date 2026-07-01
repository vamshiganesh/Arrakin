package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
)

func TestAPIKeyDevelopmentBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.APIKey(httpx.APIKeyConfig{ExpectedKey: "secret", DevBypass: true}))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 in dev bypass, got %d", resp.Code)
	}
}

func TestAPIKeyProductionRequiresMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.APIKey(httpx.APIKeyConfig{ExpectedKey: "secret", DevBypass: false}))
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "secret")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid key, got %d", resp.Code)
	}
}
