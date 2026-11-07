package middleware

import (
	"errors"
	"net/http"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
)

// BodyLimit enforces a maximum request body size using http.MaxBytesReader.
func BodyLimit(maxBytes int64, errorHandler func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil && maxBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}

		c.Next()

		// If a MaxBytesError occurred during request body reading
		for _, err := range c.Errors {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err.Err, &maxBytesErr) {
				errorHandler(c, domain.NewPayloadTooLargeError("request body exceeds maximum allowed size limit"))
				return
			}
		}
	}
}

