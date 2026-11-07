package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	internalhttp "github.com/fenet-as/KnowFlow/internal/http"
	"github.com/fenet-as/KnowFlow/internal/platform"
)

type mockDBChecker struct {
	pingErr error
}

func (m *mockDBChecker) Ping(ctx context.Context) error {
	return m.pingErr
}

func TestRouter_Healthz(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	router := internalhttp.NewRouter(internalhttp.RouterOptions{
		Logger:    logger,
		DBChecker: &mockDBChecker{},
	})

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %s", body["status"])
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID in response header")
	}
}

func TestRouter_Readyz(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	t.Run("ready when database ping succeeds", func(t *testing.T) {
		router := internalhttp.NewRouter(internalhttp.RouterOptions{
			Logger:    logger,
			DBChecker: &mockDBChecker{pingErr: nil},
		})

		req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["status"] != "ready" {
			t.Errorf("expected status 'ready', got %s", body["status"])
		}
	})

	t.Run("not ready when database ping fails", func(t *testing.T) {
		router := internalhttp.NewRouter(internalhttp.RouterOptions{
			Logger:    logger,
			DBChecker: &mockDBChecker{pingErr: errors.New("connection refused")},
		})

		req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", w.Code)
		}

		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["status"] != "not_ready" {
			t.Errorf("expected status 'not_ready', got %s", body["status"])
		}
	})
}

func TestRouter_Version(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	router := internalhttp.NewRouter(internalhttp.RouterOptions{
		Logger:    logger,
		DBChecker: &mockDBChecker{},
	})

	req, _ := http.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var info platform.VersionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("failed to unmarshal version response: %v", err)
	}

	if info.Version == "" || info.Commit == "" || info.BuildTime == "" {
		t.Errorf("expected non-empty version fields, got %+v", info)
	}
}

func TestRouter_PanicRecovery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	router := internalhttp.NewRouter(internalhttp.RouterOptions{
		Logger:    logger,
		DBChecker: &mockDBChecker{},
	})

	// Add route that triggers a panic
	router.GET("/panic", func(c *gin.Context) {
		panic("database memory allocation failed")
	})

	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Must be caught and return 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", w.Code)
	}

	var resp internalhttp.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json envelope: %v", err)
	}

	if resp.Error.Code != "internal_error" {
		t.Errorf("expected internal_error, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "An internal server error occurred. Please try again later." {
		t.Errorf("panic detail was leaked: %s", resp.Error.Message)
	}
	if resp.Error.RequestID == "" {
		t.Error("expected request_id in panic error envelope")
	}

	// Verify logger recorded the panic with stack trace
	if !bytes.Contains(buf.Bytes(), []byte("Panic recovered in HTTP handler")) {
		t.Errorf("expected panic log, got: %s", buf.String())
	}
}
