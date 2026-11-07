package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDContextKey is the context key for the unique request ID.
type requestIDContextKeyType string

const (
	RequestIDHeader                       = "X-Request-ID"
	RequestIDContextKey requestIDContextKeyType = "request_id"
)

// RequestID middleware reads or generates an X-Request-ID, assigns it to the response header,
// and stores it in the request context so downstream services can consume it without importing Gin.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if reqID == "" {
			reqID = uuid.NewString()
		}

		// Echo back in the response header
		c.Writer.Header().Set(RequestIDHeader, reqID)

		// Set in Gin context
		c.Set(string(RequestIDContextKey), reqID)

		// Set in standard request context for transport-agnostic service usage
		ctx := context.WithValue(c.Request.Context(), RequestIDContextKey, reqID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetRequestID extracts the request ID from a standard context.Context.
func GetRequestID(ctx context.Context) string {
	if val, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return val
	}
	return ""
}

