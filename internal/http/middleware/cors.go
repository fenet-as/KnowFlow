package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware enforcing a strict allowlist of allowed origins.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowMap := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		cleaned := strings.TrimRight(strings.TrimSpace(origin), "/")
		if cleaned != "" {
			allowMap[cleaned] = true
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")

		// Check if Origin is allowed
		if origin != "" && (allowMap[origin] || allowMap["*"]) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Accept, Origin")
			c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Content-Length")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		}

		// Handle preflight OPTIONS requests immediately
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

