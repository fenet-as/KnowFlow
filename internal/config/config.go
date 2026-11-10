package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all strongly-typed configuration values for KnowFlow.
type Config struct {
	// Server
	HTTPAddr string
	LogLevel string

	// Database
	DatabaseURL string

	// CORS and Auth
	CORSAllowedOrigins     []string
	ClerkIssuer            string
	ClerkAuthorizedParties []string
	JWTSecret              string
	AccessTokenDuration    time.Duration
	RefreshTokenDuration   time.Duration

	// AI Provider
	GeminiAPIKey   string
	EmbeddingModel string
	EmbeddingDim   int
	LLMModel       string

	// Ingestion & Chunking
	MaxUploadBytes    int64
	ChunkTargetTokens int
	ChunkMaxTokens    int
	ChunkOverlapPct   int

	// Retrieval & RAG
	RetrievalTopK     int
	RetrievalMinScore float64
	HistoryTurns      int

	// Workers & Jobs
	WorkerCount        int
	WorkerPollInterval time.Duration
	JobMaxAttempts     int
	JobStaleAfter      time.Duration
	JobTimeout         time.Duration

	// Rate Limiting & Timeouts
	AIRateLimitRPS float64
	AICallTimeout  time.Duration
}

// Load reads configuration from environment variables and validates required fields.
// If a .env file is found in the current working directory, it populates unset environment variables.
func Load() (*Config, error) {
	loadDotEnv()
	var missing []string

	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	clerkIssuer := getEnv("CLERK_ISSUER", "")

	geminiKey := getEnv("GEMINI_API_KEY", "")
	if geminiKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("configuration validation failed: missing required environment variables: %s", strings.Join(missing, ", "))
	}

	cfg := &Config{
		HTTPAddr:               getEnv("HTTP_ADDR", ":8080"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		DatabaseURL:            dbURL,
		CORSAllowedOrigins:     getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000"}),
		ClerkIssuer:            clerkIssuer,
		ClerkAuthorizedParties: getEnvSlice("CLERK_AUTHORIZED_PARTIES", []string{"http://localhost:5173", "http://localhost:3000"}),
		JWTSecret:              getEnv("JWT_SECRET", "dev_secret_key_knowflow_32_bytes_super_secure!"),
		AccessTokenDuration:    getEnvDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
		RefreshTokenDuration:   getEnvDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
		GeminiAPIKey:           geminiKey,
		EmbeddingModel:         getEnv("EMBEDDING_MODEL", "gemini-embedding-001"),
		EmbeddingDim:           getEnvInt("EMBEDDING_DIM", 768),
		LLMModel:               getEnv("LLM_MODEL", "gemini-3.5-flash"),
		MaxUploadBytes:         getEnvInt64("MAX_UPLOAD_BYTES", 1048576), // 1 MB
		ChunkTargetTokens:      getEnvInt("CHUNK_TARGET_TOKENS", 650),
		ChunkMaxTokens:         getEnvInt("CHUNK_MAX_TOKENS", 800),
		ChunkOverlapPct:        getEnvInt("CHUNK_OVERLAP_PCT", 12),
		RetrievalTopK:          getEnvInt("RETRIEVAL_TOP_K", 6),
		RetrievalMinScore:      getEnvFloat("RETRIEVAL_MIN_SCORE", 0.65),
		HistoryTurns:           getEnvInt("HISTORY_TURNS", 6),
		WorkerCount:            getEnvInt("WORKER_COUNT", 2),
		WorkerPollInterval:     getEnvDuration("WORKER_POLL_INTERVAL", 2*time.Second),
		JobMaxAttempts:         getEnvInt("JOB_MAX_ATTEMPTS", 5),
		JobStaleAfter:          getEnvDuration("JOB_STALE_AFTER", 10*time.Minute),
		JobTimeout:             getEnvDuration("JOB_TIMEOUT", 5*time.Minute),
		AIRateLimitRPS:         getEnvFloat("AI_RATE_LIMIT_RPS", 5.0),
		AICallTimeout:          getEnvDuration("AI_CALL_TIMEOUT", 30*time.Second),
	}

	if cfg.EmbeddingDim <= 0 {
		return nil, fmt.Errorf("invalid EMBEDDING_DIM: must be greater than 0, got %d", cfg.EmbeddingDim)
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvSlice(key string, defaultVal []string) []string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		parts := strings.Split(val, ",")
		var cleaned []string
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return defaultVal
}

// loadDotEnv reads key-value pairs from a local .env file if it exists,
// populating any environment variables that are not already set.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
				v = v[1 : len(v)-1]
			}
			if _, exists := os.LookupEnv(k); !exists {
				os.Setenv(k, v)
			}
		}
	}
}
