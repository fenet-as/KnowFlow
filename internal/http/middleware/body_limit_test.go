package middleware_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestBodyLimit(t *testing.T) {
	mockErrorHandler := func(c *gin.Context, err error) {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload_too_large"})
	}

	r := gin.New()
	r.Use(middleware.BodyLimit(100, mockErrorHandler)) // 100 bytes limit
	r.POST("/upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.String(http.StatusOK, "read successfully")
	})

	t.Run("body within limit succeeds", func(t *testing.T) {
		smallBody := bytes.Repeat([]byte("a"), 50)
		req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewReader(smallBody))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("body exceeding limit returns 413", func(t *testing.T) {
		largeBody := bytes.Repeat([]byte("a"), 200)
		req, _ := http.NewRequest(http.MethodPost, "/upload", bytes.NewReader(largeBody))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected 413 Payload Too Large, got %d", w.Code)
		}
	})
}

