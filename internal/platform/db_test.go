package platform_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/platform"
)

func getTestDatabaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable"
	}
	return url
}

func TestNewDBPool_ConnectsAndValidates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbURL := getTestDatabaseURL()
	poolCfg := platform.DefaultDBPoolConfig()
	pool, err := platform.NewDBPool(ctx, dbURL, poolCfg)
	if err != nil {
		t.Skipf("PostgreSQL database not available on %s: %v", dbURL, err)
		return
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("expected ping to succeed, got: %v", err)
	}

	// Test ValidateEmbeddingDimension with matching dimension (768)
	if err := platform.ValidateEmbeddingDimension(ctx, pool, 768); err != nil {
		t.Fatalf("expected 768 dimension to validate, got: %v", err)
	}

	// Test ValidateEmbeddingDimension with mismatch (e.g., 1536)
	errMismatch := platform.ValidateEmbeddingDimension(ctx, pool, 1536)
	if errMismatch == nil {
		t.Fatalf("expected error for mismatched dimension 1536, got nil")
	}
}

