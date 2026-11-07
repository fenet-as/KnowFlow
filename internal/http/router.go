package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/gin-gonic/gin"
)

// DBChecker defines the contract for testing database connectivity.
type DBChecker interface {
	Ping(ctx context.Context) error
}

// RouterOptions groups dependencies needed by the HTTP router.
type RouterOptions struct {
	Logger         *slog.Logger
	DBChecker      DBChecker
	CORSOrigins    []string
	MaxUploadBytes int64
	ClerkVerifier  *auth.ClerkVerifier
}

// NewRouter initializes the Gin engine with strict middleware ordering and route groups.
func NewRouter(opts RouterOptions) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 1. Request ID (sets/propagates X-Request-ID in context and header)
	r.Use(middleware.RequestID())

	// 2. Panic recovery (recovers panics, logs stack trace, returns generic 500 envelope)
	r.Use(middleware.Recovery(opts.Logger))

	// 3. Structured access logger using slog
	r.Use(middleware.AccessLogger(opts.Logger))

	// 4. Strict CORS allowlist from configuration
	if len(opts.CORSOrigins) > 0 {
		r.Use(middleware.CORS(opts.CORSOrigins))
	}

	// 5. Body limit enforcement (sized from MaxUploadBytes + overhead)
	if opts.MaxUploadBytes > 0 {
		// Provide 64KB overhead for multipart form boundaries and metadata
		limitWithOverhead := opts.MaxUploadBytes + (64 * 1024)
		r.Use(middleware.BodyLimit(limitWithOverhead, RespondWithError))
	}

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

	// Protected business routes group (/api/v1)
	apiV1 := r.Group("/api/v1")
	if opts.ClerkVerifier != nil {
		apiV1.Use(auth.Middleware(opts.ClerkVerifier, RespondWithError))
	}

	// Example protected identity verification route
	apiV1.GET("/me", func(c *gin.Context) {
		ownerID := auth.GetOwnerID(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{
			"owner_id": ownerID,
		})
	})

	return r
}
