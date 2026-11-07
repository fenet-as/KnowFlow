package middleware

import (
	"log/slog"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/gin-gonic/gin"
)

// AccessLogger produces one structured JSON log line per request using log/slog.
// It includes method, path, status, duration, request ID, and owner ID when known.
func AccessLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		reqID := GetRequestID(c.Request.Context())
		ownerID := auth.GetOwnerID(c.Request.Context())

		fullPath := path
		if rawQuery != "" {
			fullPath = path + "?" + rawQuery
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", fullPath,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"request_id", reqID,
		}

		if ownerID != "" {
			attrs = append(attrs, "owner_id", ownerID)
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		if status >= 500 {
			logger.Error("HTTP request failed", attrs...)
		} else if status >= 400 {
			logger.Warn("HTTP request client error", attrs...)
		} else {
			logger.Info("HTTP request", attrs...)
		}
	}
}

