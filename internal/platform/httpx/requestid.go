package httpx

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const requestIDKey ctxKey = 1

const headerRequestID = "X-Request-Id"

// RequestIDHeader is the canonical header name for request correlation.
const RequestIDHeader = headerRequestID

// WithRequestID stores request id in context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns request id or empty string.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// EnsureRequestID returns id from context or generates a new one.
func EnsureRequestID(ctx context.Context) (context.Context, string) {
	if id := RequestIDFromContext(ctx); id != "" {
		return ctx, id
	}
	id := uuid.NewString()
	return WithRequestID(ctx, id), id
}
