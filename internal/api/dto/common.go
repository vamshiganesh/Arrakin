package dto

import "github.com/vamshiganesh/arrakin/internal/platform/httpx"

// PageMeta re-exports pagination metadata for API responses.
type PageMeta = httpx.PageMeta

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}
