package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vamshiganesh/arrakin/internal/idempotency"
	"github.com/vamshiganesh/arrakin/internal/platform/httpx"
	"github.com/vamshiganesh/arrakin/internal/store"
	"github.com/vamshiganesh/arrakin/internal/store/sqlc"
)

const idempotencyHeader = "Idempotency-Key"

// Idempotency executes mutating handlers with idempotency key deduplication.
func Idempotency(st *store.Store, svc *idempotency.Service, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader(idempotencyHeader)
		if key == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, httpx.ErrorBody{
				Error:   "Idempotency-Key header is required",
				Code:    "validation_error",
			})
			return
		}

		var response idempotency.StoredResponse
		var replayed bool
		err := st.WithTx(c.Request.Context(), func(ctx context.Context, q *sqlc.Queries) error {
			var execErr error
			response, replayed, execErr = svc.Execute(ctx, q, scope, key, "", func(ctx context.Context) (idempotency.StoredResponse, error) {
				recorder := &responseRecorder{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
				c.Writer = recorder
				c.Next()

				if recorder.status == 0 {
					recorder.status = c.Writer.Status()
				}
				if recorder.status >= 500 {
					return idempotency.StoredResponse{}, httpx.ErrBadRequest
				}

				return idempotency.StoredResponse{
					StatusCode: recorder.status,
					Body:       json.RawMessage(recorder.body.Bytes()),
				}, nil
			})
			return execErr
		})
		if err != nil {
			httpx.WriteError(c, err)
			c.Abort()
			return
		}

		if replayed {
			c.Header("X-Idempotency-Replayed", "true")
		}
		c.Data(response.StatusCode, "application/json", response.Body)
		c.Abort()
	}
}

type responseRecorder struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

func (r *responseRecorder) WriteString(s string) (int, error) {
	return r.Write([]byte(s))
}

// DrainBody is a no-op helper kept for future request-hash support.
func DrainBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	return io.ReadAll(c.Request.Body)
}
