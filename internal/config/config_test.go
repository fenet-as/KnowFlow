package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/config"
)

func setRequiredEnvs(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://knowflow:knowflow@localhost:5432/knowflow?sslmode=disable")
	t.Setenv("CLERK_ISSUER", "https://clerk.example.com")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")
}

func TestLoad_SuccessWithDefaults(t *testing.T) {
	setRequiredEnvs(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("expected default HTTPAddr :8080, got %s", cfg.HTTPAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel info, got %s", cfg.LogLevel)
	}
	if cfg.EmbeddingDim != 768 {
		t.Errorf("expected default EmbeddingDim 768, got %d", cfg.EmbeddingDim)
	}
	if cfg.MaxUploadBytes != 1048576 {
		t.Errorf("expected default MaxUploadBytes 1048576, got %d", cfg.MaxUploadBytes)
	}
	if cfg.WorkerCount != 2 {
		t.Errorf("expected default WorkerCount 2, got %d", cfg.WorkerCount)
	}
	if cfg.WorkerPollInterval != 2*time.Second {
		t.Errorf("expected default WorkerPollInterval 2s, got %v", cfg.WorkerPollInterval)
	}
	if cfg.RetrievalMinScore != 0.65 {
		t.Errorf("expected default RetrievalMinScore 0.65, got %f", cfg.RetrievalMinScore)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Errorf("expected 2 default CORS origins, got %d", len(cfg.CORSAllowedOrigins))
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	testCases := []struct {
		name          string
		clearEnvs     []string
		expectedError string
	}{
		{
			name:          "missing DATABASE_URL",
			clearEnvs:     []string{"DATABASE_URL"},
			expectedError: "DATABASE_URL",
		},
		{
			name:          "missing CLERK_ISSUER",
			clearEnvs:     []string{"CLERK_ISSUER"},
			expectedError: "CLERK_ISSUER",
		},
		{
			name:          "missing GEMINI_API_KEY",
			clearEnvs:     []string{"GEMINI_API_KEY"},
			expectedError: "GEMINI_API_KEY",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnvs(t)
			for _, env := range tc.clearEnvs {
				_ = os.Unsetenv(env)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.expectedError)
			}
			if !contains(err.Error(), tc.expectedError) {
				t.Errorf("expected error %q to contain %q", err.Error(), tc.expectedError)
			}
		})
	}
}

func TestLoad_CustomOverrides(t *testing.T) {
	setRequiredEnvs(t)
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("EMBEDDING_DIM", "1536")
	t.Setenv("WORKER_COUNT", "4")
	t.Setenv("WORKER_POLL_INTERVAL", "500ms")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.knowflow.io, https://staging.knowflow.io")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Errorf("expected HTTPAddr :9090, got %s", cfg.HTTPAddr)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug, got %s", cfg.LogLevel)
	}
	if cfg.EmbeddingDim != 1536 {
		t.Errorf("expected EmbeddingDim 1536, got %d", cfg.EmbeddingDim)
	}
	if cfg.WorkerCount != 4 {
		t.Errorf("expected WorkerCount 4, got %d", cfg.WorkerCount)
	}
	if cfg.WorkerPollInterval != 500*time.Millisecond {
		t.Errorf("expected WorkerPollInterval 500ms, got %v", cfg.WorkerPollInterval)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != "https://app.knowflow.io" {
		t.Errorf("expected custom CORS origins parsed, got %v", cfg.CORSAllowedOrigins)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 || (len(haystack) > 0 && len(needle) > 0 && stringContains(haystack, needle)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

