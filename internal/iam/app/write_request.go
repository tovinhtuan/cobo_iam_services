package app

import "context"

type writeRequestKey struct{}

// WithWriteRequest marks a request as one that changes company data. The HTTP layer sets it;
// token inspection refuses such requests in a suspended ("Tạm ngưng") company.
func WithWriteRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, writeRequestKey{}, true)
}

// IsWriteRequest reports whether WithWriteRequest marked the request.
func IsWriteRequest(ctx context.Context) bool {
	v, _ := ctx.Value(writeRequestKey{}).(bool)
	return v
}
