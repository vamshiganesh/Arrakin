package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/store"
)

// ErrorBody is the standard API error envelope.
type ErrorBody struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// WriteError maps domain errors to HTTP responses.
func WriteError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	switch {
	case store.IsNotFound(err):
		c.JSON(http.StatusNotFound, ErrorBody{Error: "not found", Code: "not_found"})
	case errors.Is(err, store.ErrConflict):
		c.JSON(http.StatusConflict, ErrorBody{Error: "state conflict", Code: "conflict", Details: err.Error()})
	case errors.Is(err, store.ErrNoJobAvailable):
		c.JSON(http.StatusNotFound, ErrorBody{Error: "no job available", Code: "not_found"})
	case errors.Is(err, idempotency.ErrKeyInProgress):
		c.JSON(http.StatusConflict, ErrorBody{Error: "idempotency key in progress", Code: "idempotency_in_progress"})
	case errors.Is(err, ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorBody{Error: "unauthorized", Code: "unauthorized"})
	case errors.Is(err, ErrBadRequest):
		c.JSON(http.StatusBadRequest, ErrorBody{Error: "bad request", Code: "bad_request", Details: err.Error()})
	case errors.Is(err, ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorBody{Error: "validation failed", Code: "validation_error", Details: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, ErrorBody{Error: "internal server error", Code: "internal_error"})
	}
}
