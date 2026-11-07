package http

import (
	"log/slog"
	"net/http"

	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/gin-gonic/gin"
)

// NewRouter initializes the Gin engine with strict middleware ordering.
func NewRouter(logger *slog.Logger) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create engine without default logger or recovery to have explicit control
	r := gin.New()

	// 1. Request ID middleware (generates/propagates X-Request-ID)
	r.Use(middleware.RequestID())

	// 2. Structured access logger using slog
	r.Use(middleware.AccessLogger(logger))

	// Unauthenticated operational routes
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

