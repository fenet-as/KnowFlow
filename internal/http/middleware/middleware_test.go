package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestID_GeneratesAndEchoes(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/test", func(c *gin.Context) {
		reqID := middleware.GetRequestID(c.Request.Context())
		if reqID == "" {
			t.Error("expected request ID in context, got empty")
		}
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respHeader := w.Header().Get(middleware.RequestIDHeader)
	if respHeader == "" {
		t.Fatal("expected X-Request-ID header in response, got none")
	}
}

func TestRequestID_PreservesExistingHeader(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/test", func(c *gin.Context) {
		reqID := middleware.GetRequestID(c.Request.Context())
		if reqID != "client-req-123" {
			t.Errorf("expected preserved request ID 'client-req-123', got %s", reqID)
		}
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(middleware.RequestIDHeader, "client-req-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respHeader := w.Header().Get(middleware.RequestIDHeader)
	if respHeader != "client-req-123" {
		t.Fatalf("expected preserved X-Request-ID 'client-req-123', got %s", respHeader)
	}
}

func TestAccessLogger_LogsStructuredJSON(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.AccessLogger(logger))

	// Mock endpoint with owner ID in context
	r.GET("/protected", func(c *gin.Context) {
		ctx := auth.WithOwnerID(c.Request.Context(), "user_test_999")
		c.Request = c.Request.WithContext(ctx)
		c.JSON(http.StatusOK, gin.H{"msg": "success"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/protected?page=1", nil)
	req.Header.Set(middleware.RequestIDHeader, "custom-id-456")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var logMap map[string]any
	if err := json.Unmarshal(logBuf.Bytes(), &logMap); err != nil {
		t.Fatalf("expected valid JSON log, got error: %v, raw output: %s", err, logBuf.String())
	}

	if logMap["method"] != "GET" {
		t.Errorf("expected method GET, got %v", logMap["method"])
	}
	if logMap["path"] != "/protected?page=1" {
		t.Errorf("expected path '/protected?page=1', got %v", logMap["path"])
	}
	if logMap["request_id"] != "custom-id-456" {
		t.Errorf("expected request_id 'custom-id-456', got %v", logMap["request_id"])
	}
	if logMap["owner_id"] != "user_test_999" {
		t.Errorf("expected owner_id 'user_test_999', got %v", logMap["owner_id"])
	}
}

