package auth

// ContextKey represents typed keys for context values.
type ContextKey string

const (
	// OwnerIDContextKey is the context key for authenticated owner ID (Clerk subject claim).
	OwnerIDContextKey ContextKey = "owner_id"
)

