package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestGeminiEmbedder_Success(t *testing.T) {
	expectedDim := 768
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1beta/models/text-embedding-004:batchEmbedContents", r.URL.Path)
		assert.Equal(t, "test-api-key", r.Header.Get("x-goog-api-key"))

		var reqPayload geminiBatchEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&reqPayload)
		require.NoError(t, err)
		assert.Len(t, reqPayload.Requests, 2)
		assert.Equal(t, "RETRIEVAL_DOCUMENT", reqPayload.Requests[0].TaskType)
		assert.Equal(t, "models/text-embedding-004", reqPayload.Requests[0].Model)

		// Create mock response
		resp := geminiBatchEmbedResponse{
			Embeddings: []geminiEmbeddingValues{
				{Values: make([]float32, expectedDim)},
				{Values: make([]float32, expectedDim)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:    "test-api-key",
		Model:     "text-embedding-004",
		Dimension: expectedDim,
		BaseURL:   server.URL,
	})
	require.NoError(t, err)
	assert.Equal(t, expectedDim, embedder.Dimension())

	ctx := context.Background()
	vecs, err := embedder.Embed(ctx, []string{"first chunk", "second chunk"}, EmbedKindDocument)
	require.NoError(t, err)
	assert.Len(t, vecs, 2)
	assert.Len(t, vecs[0], expectedDim)
	assert.Len(t, vecs[1], expectedDim)
}

func TestGeminiEmbedder_QueryTaskType(t *testing.T) {
	expectedDim := 768
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqPayload geminiBatchEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&reqPayload)
		require.NoError(t, err)
		assert.Equal(t, "RETRIEVAL_QUERY", reqPayload.Requests[0].TaskType)

		resp := geminiBatchEmbedResponse{
			Embeddings: []geminiEmbeddingValues{
				{Values: make([]float32, expectedDim)},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:    "test-api-key",
		Dimension: expectedDim,
		BaseURL:   server.URL,
	})
	require.NoError(t, err)

	queryVec, err := EmbedQuery(context.Background(), embedder, "what is knowflow?")
	require.NoError(t, err)
	assert.Len(t, queryVec, expectedDim)
}

func TestGeminiEmbedder_DimensionMismatchIsPermanentError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return dimension 256 instead of expected 768
		resp := geminiBatchEmbedResponse{
			Embeddings: []geminiEmbeddingValues{
				{Values: make([]float32, 256)},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:    "test-api-key",
		Dimension: 768,
		BaseURL:   server.URL,
	})
	require.NoError(t, err)

	_, err = embedder.Embed(context.Background(), []string{"test text"}, EmbedKindDocument)
	assert.Error(t, err)
	assert.True(t, domain.IsPermanent(err), "expected error to be permanent")
}

func TestGeminiEmbedder_RateLimitIsTransientError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"code": 429, "message": "Resource exhausted"}}`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = embedder.Embed(context.Background(), []string{"test text"}, EmbedKindDocument)
	assert.Error(t, err)
	assert.True(t, domain.IsTransient(err), "expected 429 to be transient")
}

func TestGeminiEmbedder_ServerUnavailableIsTransientError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": "server temporarily overloaded"}`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = embedder.Embed(context.Background(), []string{"test text"}, EmbedKindDocument)
	assert.Error(t, err)
	assert.True(t, domain.IsTransient(err), "expected 503 to be transient")
}

func TestGeminiEmbedder_BadRequestIsPermanentError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"code": 400, "message": "Invalid argument: model not found"}}`))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = embedder.Embed(context.Background(), []string{"test text"}, EmbedKindDocument)
	assert.Error(t, err)
	assert.True(t, domain.IsPermanent(err), "expected 400 to be permanent")
}

func TestGeminiEmbedder_TimeoutIsTransientError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Millisecond},
	})
	require.NoError(t, err)

	_, err = embedder.Embed(context.Background(), []string{"test text"}, EmbedKindDocument)
	assert.Error(t, err)
	assert.True(t, domain.IsTransient(err), "expected timeout to be transient")
}

func TestGeminiEmbedder_BatchingAbove100(t *testing.T) {
	expectedDim := 768
	batchCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchCalls++
		var reqPayload geminiBatchEmbedRequest
		_ = json.NewDecoder(r.Body).Decode(&reqPayload)

		resp := geminiBatchEmbedResponse{
			Embeddings: make([]geminiEmbeddingValues, len(reqPayload.Requests)),
		}
		for i := range resp.Embeddings {
			resp.Embeddings[i].Values = make([]float32, expectedDim)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:    "test-api-key",
		Dimension: expectedDim,
		BaseURL:   server.URL,
	})
	require.NoError(t, err)

	// Send 150 texts (should trigger 2 batch requests: 100 + 50)
	texts := make([]string, 150)
	for i := range texts {
		texts[i] = fmt.Sprintf("chunk passage %d", i)
	}

	vecs, err := embedder.Embed(context.Background(), texts, EmbedKindDocument)
	require.NoError(t, err)
	assert.Equal(t, 2, batchCalls)
	assert.Len(t, vecs, 150)
}

func TestGeminiEmbedder_SharedRateLimiter(t *testing.T) {
	expectedDim := 768
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiBatchEmbedResponse{
			Embeddings: []geminiEmbeddingValues{{Values: make([]float32, expectedDim)}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Rate limiter with 1 RPS and burst 1
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:    "test-api-key",
		Dimension: expectedDim,
		BaseURL:   server.URL,
		Limiter:   limiter,
	})
	require.NoError(t, err)

	start := time.Now()
	// First call consumes the burst token immediately
	_, err = embedder.Embed(context.Background(), []string{"test 1"}, EmbedKindDocument)
	require.NoError(t, err)

	// Second call must wait ~100ms for the limiter
	_, err = embedder.Embed(context.Background(), []string{"test 2"}, EmbedKindDocument)
	require.NoError(t, err)
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 80*time.Millisecond, "rate limiter should pace requests")
}

func TestGeminiEmbedder_RetryAfterHonored(t *testing.T) {
	expectedDim := 768
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call returns 429 with 1-second Retry-After
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
			return
		}

		// Second call succeeds
		resp := geminiBatchEmbedResponse{
			Embeddings: []geminiEmbeddingValues{{Values: make([]float32, expectedDim)}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	embedder, err := NewGeminiEmbedder(GeminiEmbedderConfig{
		APIKey:    "test-api-key",
		Dimension: expectedDim,
		BaseURL:   server.URL,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)

	start := time.Now()
	vecs, err := embedder.Embed(context.Background(), []string{"retry chunk"}, EmbedKindDocument)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Len(t, vecs, 1)
	assert.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond, "should honor 1s Retry-After")
}
