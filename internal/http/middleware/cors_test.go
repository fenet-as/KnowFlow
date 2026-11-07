package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestCORS_Allowlist(t *testing.T) {
	allowed := []string{"http://localhost:5173", "https://app.knowflow.io"}

	r := gin.New()
	r.Use(middleware.CORS(allowed))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	t.Run("allowed origin receives headers", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Errorf("expected CORS header, got: %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("disallowed origin receives no CORS header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", "https://malicious.example.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected no CORS header for disallowed origin, got: %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("preflight OPTIONS request returns 204 No Content", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
		req.Header.Set("Origin", "https://app.knowflow.io")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "https://app.knowflow.io" {
			t.Errorf("expected CORS header on OPTIONS, got: %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})
}

