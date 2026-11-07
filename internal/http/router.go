package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/gin-gonic/gin"
)

// DBChecker defines the contract for testing database connectivity.
// *pgxpool.Pool satisfies this interface.
type DBChecker interface {
	Ping(ctx context.Context) error
}

// RouterOptions groups dependencies needed by the HTTP router.
type RouterOptions struct {
	Logger    *slog.Logger
	DBChecker DBChecker
}

// NewRouter initializes the Gin engine with strict middleware ordering and operational routes.
func NewRouter(opts RouterOptions) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 1. Request ID (sets/propagates X-Request-ID in context and header)
	r.Use(middleware.RequestID())

	// 2. Panic recovery (recovers panics, logs stack trace, returns generic 500 envelope)
	r.Use(middleware.Recovery(opts.Logger))

	// 3. Structured access logger using slog
	r.Use(middleware.AccessLogger(opts.Logger))

	// Unauthenticated operational endpoints (KF-08)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if opts.DBChecker == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database checker not configured",
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if err := opts.DBChecker.Ping(ctx); err != nil {
			opts.Logger.Warn("Readiness probe failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database unavailable",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, platform.GetVersionInfo())
	})

	return r
}
