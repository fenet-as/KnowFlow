package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestGeminiLLM_GenerateSuccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1beta/models/gemini-1.5-flash:generateContent", r.URL.Path)
		assert.Equal(t, "test-api-key", r.Header.Get("x-goog-api-key"))

		var reqPayload geminiGenerateRequest
		err := json.NewDecoder(r.Body).Decode(&reqPayload)
		require.NoError(t, err)
		assert.Len(t, reqPayload.Contents, 1)
		assert.Equal(t, "user prompt", reqPayload.Contents[0].Parts[0].Text)

		resp := geminiGenerateResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{Text: "Synthesized factual answer [1]."},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:  "test-api-key",
		Model:   "gemini-1.5-flash",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	answer, err := gemini.Generate(context.Background(), "user prompt")
	require.NoError(t, err)
	assert.Equal(t, "Synthesized factual answer [1].", answer)
}

func TestGeminiLLM_RateLimitIsTransient(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = gemini.Generate(context.Background(), "prompt")
	assert.Error(t, err)
	assert.True(t, domain.IsTransient(err))
}

func TestGeminiLLM_ServiceUnavailableIsTransient(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = gemini.Generate(context.Background(), "prompt")
	assert.Error(t, err)
	assert.True(t, domain.IsTransient(err))
}

func TestGeminiLLM_BadRequestIsPermanent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid model"}`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = gemini.Generate(context.Background(), "prompt")
	assert.Error(t, err)
	assert.True(t, domain.IsPermanent(err))
}

func TestGeminiLLM_TimeoutIsTransient(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Millisecond},
	})
	require.NoError(t, err)

	_, err = gemini.Generate(context.Background(), "prompt")
	assert.Error(t, err)
	assert.True(t, domain.IsTransient(err))
}

func TestFakeLLM(t *testing.T) {
	fake := NewFakeLLM("Initial response [1]")
	assert.Equal(t, 0, fake.Calls())

	ans, err := fake.Generate(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, "Initial response [1]", ans)
	assert.Equal(t, 1, fake.Calls())
	assert.Equal(t, "hello", fake.LastPrompt())

	// Dynamic response func
	fake.SetResponseFunc(func(p string) string {
		return "Answer for: " + p
	})
	ans2, err := fake.Generate(context.Background(), "prompt2")
	require.NoError(t, err)
	assert.Equal(t, "Answer for: prompt2", ans2)
}

func TestGeminiLLM_SharedRateLimiter(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiGenerateResponse{
			Candidates: []geminiCandidate{
				{Content: geminiContent{Parts: []geminiPart{{Text: "Paced answer"}}}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Limiter: limiter,
	})
	require.NoError(t, err)

	start := time.Now()
	// First call consumes the burst token immediately
	_, err = gemini.Generate(context.Background(), "q1")
	require.NoError(t, err)

	// Second call must wait ~100ms for the limiter
	_, err = gemini.Generate(context.Background(), "q2")
	require.NoError(t, err)
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 80*time.Millisecond, "rate limiter should pace requests")
}

func TestGeminiLLM_RetryAfterHonored(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
			return
		}

		resp := geminiGenerateResponse{
			Candidates: []geminiCandidate{
				{Content: geminiContent{Parts: []geminiPart{{Text: "Recovered answer"}}}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	gemini, err := NewGeminiLLM(GeminiLLMConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)

	start := time.Now()
	ans, err := gemini.Generate(context.Background(), "prompt")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, "Recovered answer", ans)
	assert.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond, "should honor 1s Retry-After")
}
