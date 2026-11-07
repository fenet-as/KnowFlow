package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fenet-as/KnowFlow/internal/config"
	"github.com/fenet-as/KnowFlow/internal/platform"
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

	// Set up basic HTTP handler for server
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, `{"status":"ok"}`)
	})

	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors coming from the listener
	serverErrors := make(chan error, 1)

	// Start server in background
	go func() {
		logger.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	// Channel to listen for interrupt or terminate signal from the OS
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

		// Context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Graceful shutdown failed, forcing close", "error", err)
			_ = server.Close()
		}
		logger.Info("Server stopped cleanly")
	}
}
