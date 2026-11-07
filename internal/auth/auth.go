package auth

import "context"

// ContextKey represents typed keys for context values.
type ContextKey string

const (
	// OwnerIDContextKey is the context key for authenticated owner ID (Clerk subject claim).
	OwnerIDContextKey ContextKey = "owner_id"
)

// WithOwnerID returns a context with the given owner ID set.
func WithOwnerID(ctx context.Context, ownerID string) context.Context {
	return context.WithValue(ctx, OwnerIDContextKey, ownerID)
}

// GetOwnerID extracts the authenticated owner ID from the context, or returns an empty string.
func GetOwnerID(ctx context.Context) string {
	if val, ok := ctx.Value(OwnerIDContextKey).(string); ok {
		return val
	}
	return ""
}
