package platform

import (
	"log/slog"
	"os"
)

// SetupLogger initializes the application's structured slog JSON logger.
func SetupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

