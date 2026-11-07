package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery middleware recovers from any runtime panics, logs the error with stack trace via slog,
// and returns a client-safe generic 500 error envelope with the active request ID.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				reqID := GetRequestID(c.Request.Context())
				stack := string(debug.Stack())

				// Log structured panic details with stack trace
				logger.Error("Panic recovered in HTTP handler",
					"error", fmt.Sprintf("%v", r),
					"stack", stack,
					"request_id", reqID,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)

				// Standard safe error envelope (no internal details leaked)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":       "internal_error",
						"message":    "An internal server error occurred. Please try again later.",
						"request_id": reqID,
					},
				})
			}
		}()

		c.Next()
	}
}

