package requestid

import (
	"context"

	"github.com/google/uuid"
)

const Header = "X-Request-Id"

type contextKey struct{}

func New() string {
	return uuid.NewString()
}

func WithContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKey{}, requestID)
}

func FromContext(ctx context.Context) string {
	value, ok := ctx.Value(contextKey{}).(string)
	if !ok {
		return ""
	}

	return value
}
