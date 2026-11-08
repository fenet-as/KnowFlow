package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/config"
	"github.com/fenet-as/KnowFlow/internal/embed"
	internalhttp "github.com/fenet-as/KnowFlow/internal/http"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/fenet-as/KnowFlow/internal/service"
)

func main() {
	// Base logger for bootstrapping
	baseLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(baseLogger)

	// Load configuration (fails fast on missing required variables)
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Configuration error: failing fast", "error", err)
		os.Exit(1)
	}

	// Reconfigure structured logger
	logger := platform.SetupLogger(cfg.LogLevel)
	logger.Info("Configuration loaded successfully",
		"http_addr", cfg.HTTPAddr,
		"log_level", cfg.LogLevel,
		"embedding_model", cfg.EmbeddingModel,
		"embedding_dim", cfg.EmbeddingDim,
		"llm_model", cfg.LLMModel,
		"worker_count", cfg.WorkerCount,
	)

	// Context for database connection and initialization
	initCtx, initCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer initCancel()

	// Initialize PostgreSQL connection pool with timeouts and pgvector types
	poolCfg := platform.DefaultDBPoolConfig()
	dbPool, err := platform.NewDBPool(initCtx, cfg.DatabaseURL, poolCfg)
	if err != nil {
		logger.Error("Database connection pool failed", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Validate embedding dimension against database chunks table
	if err := platform.ValidateEmbeddingDimension(initCtx, dbPool, cfg.EmbeddingDim); err != nil {
		logger.Error("Embedding dimension validation failed", "error", err)
		os.Exit(1)
	}
	logger.Info("Database pool connected and embedding dimension validated", "dim", cfg.EmbeddingDim)

	// Initialize Clerk JWT session verifier
	clerkVerifier, err := auth.NewClerkVerifier(auth.ClerkVerifierOptions{
		Issuer:            cfg.ClerkIssuer,
		AuthorizedParties: cfg.ClerkAuthorizedParties,
	})
	if err != nil {
		logger.Error("Failed to initialize Clerk verifier", "error", err)
		os.Exit(1)
	}

	// Initialize repositories and services
	docRepo := repo.NewPostgresDocumentRepo(dbPool)
	chunkRepo := repo.NewPostgresChunkRepo(dbPool)

	docService := service.NewDocumentService(docRepo, service.DocumentServiceConfig{
		MaxUploadBytes: cfg.MaxUploadBytes,
		JobMaxAttempts: cfg.JobMaxAttempts,
	})
	docHandler := internalhttp.NewDocumentHandler(docService, cfg.MaxUploadBytes)

	// Initialize Gemini embedder and retrieval service
	geminiEmbedder, err := embed.NewGeminiEmbedder(embed.GeminiEmbedderConfig{
		APIKey:    cfg.GeminiAPIKey,
		Model:     cfg.EmbeddingModel,
		Dimension: cfg.EmbeddingDim,
		Timeout:   cfg.AICallTimeout,
	})
	if err != nil {
		logger.Error("Failed to initialize Gemini embedder", "error", err)
		os.Exit(1)
	}

	retrievalService := service.NewRetrievalService(geminiEmbedder, chunkRepo, service.RetrievalConfig{
		TopK:     cfg.RetrievalTopK,
		MinScore: cfg.RetrievalMinScore,
	}, logger)
	_ = retrievalService // Prepared for KF-20 (Stateless Ask) and KF-21 (Chunk detail)

	// Initialize Gin HTTP router with Request ID, Recovery, Access Logger, CORS, Limits, and Probes
	router := internalhttp.NewRouter(internalhttp.RouterOptions{
		Logger:          logger,
		DBChecker:       dbPool,
		CORSOrigins:     cfg.CORSAllowedOrigins,
		MaxUploadBytes:  cfg.MaxUploadBytes,
		ClerkVerifier:   clerkVerifier,
		DocumentHandler: docHandler,
	})

	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to capture server runtime errors
	serverErrors := make(chan error, 1)

	// Start server in background
	go func() {
		logger.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	// Channel for graceful shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until signal or error
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	case sig := <-shutdown:
		logger.Info("Shutdown signal received", "signal", sig.String())

		// Graceful shutdown with 10s deadline
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Graceful shutdown failed, forcing close", "error", err)
			_ = server.Close()
		}
		logger.Info("Server stopped cleanly")
	}
}
