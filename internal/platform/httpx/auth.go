package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrUnauthorized is returned when API key validation fails.
var ErrUnauthorized = errors.New("unauthorized")

const apiKeyHeader = "X-API-Key"

// APIKeyConfig controls admin API key enforcement.
type APIKeyConfig struct {
	ExpectedKey string
	DevBypass   bool
}

// APIKey enforces X-API-Key on protected routes.
// In development mode (DevBypass=true), requests without a key are allowed.
func APIKey(cfg APIKeyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader(apiKeyHeader)
		if cfg.DevBypass {
			if key == "" || (cfg.ExpectedKey != "" && key == cfg.ExpectedKey) {
				c.Next()
				return
			}
		}
		if cfg.ExpectedKey == "" || key != cfg.ExpectedKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorBody{
				Error: "unauthorized",
				Code:  "unauthorized",
			})
			return
		}
		c.Next()
	}
}
