package platform

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

// DBPoolConfig defines parameters for tuning the pgx connection pool.
type DBPoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

// DefaultDBPoolConfig returns battle-tested default settings for PostgreSQL pool.
func DefaultDBPoolConfig() DBPoolConfig {
	return DBPoolConfig{
		MaxConns:          25,
		MinConns:          2,
		MaxConnLifetime:   1 * time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
		ConnectTimeout:    10 * time.Second,
	}
}

// NewDBPool initializes a pgxpool.Pool with connection timeouts and registers pgvector types.
func NewDBPool(ctx context.Context, databaseURL string, poolCfg DBPoolConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}

	config.MaxConns = poolCfg.MaxConns
	config.MinConns = poolCfg.MinConns
	config.MaxConnLifetime = poolCfg.MaxConnLifetime
	config.MaxConnIdleTime = poolCfg.MaxConnIdleTime
	config.HealthCheckPeriod = poolCfg.HealthCheckPeriod

	// Register pgvector types on every new connection
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	connectCtx, cancel := context.WithTimeout(ctx, poolCfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	// Verify database connectivity
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// ValidateEmbeddingDimension validates that the chunks table embedding vector dimension matches expectedDim.
// If the table does not exist yet (e.g. before initial migration), validation succeeds.
func ValidateEmbeddingDimension(ctx context.Context, pool *pgxpool.Pool, expectedDim int) error {
	query := `
		SELECT atttypmod
		FROM pg_attribute
		WHERE attrelid = 'chunks'::regclass
		  AND attname = 'embedding'
		  AND NOT attisdropped;
	`

	var atttypmod int
	err := pool.QueryRow(ctx, query).Scan(&atttypmod)
	if err != nil {
		// Table or column doesn't exist yet
		slog.Debug("Chunks table embedding column not found in catalog (possibly unmigrated yet), skipping dimension validation", "error", err)
		return nil
	}

	// In pgvector, atttypmod stores the exact dimension
	if atttypmod != expectedDim {
		return fmt.Errorf("database chunks.embedding dimension (%d) does not match configured EMBEDDING_DIM (%d)", atttypmod, expectedDim)
	}

	return nil
}

