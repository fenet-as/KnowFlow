package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"golang.org/x/time/rate"
)

const (
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"
	geminiMaxBatchSize   = 100
)

// GeminiEmbedderConfig holds configuration parameters for the Gemini embedder.
type GeminiEmbedderConfig struct {
	APIKey     string
	Model      string
	Dimension  int
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	Limiter    *rate.Limiter
}

// GeminiEmbedder generates vector embeddings using Google's Gemini REST API.
type GeminiEmbedder struct {
	apiKey     string
	model      string
	dimension  int
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	limiter    *rate.Limiter
}

// NewGeminiEmbedder constructs a new GeminiEmbedder and validates required fields.
func NewGeminiEmbedder(cfg GeminiEmbedderConfig) (*GeminiEmbedder, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, domain.NewPermanentError("gemini API key is required", domain.ErrValidation)
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gemini-embedding-001"
	}

	dimension := cfg.Dimension
	if dimension <= 0 {
		dimension = 768
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	return &GeminiEmbedder{
		apiKey:     cfg.APIKey,
		model:      model,
		dimension:  dimension,
		baseURL:    baseURL,
		httpClient: client,
		timeout:    timeout,
		limiter:    cfg.Limiter,
	}, nil
}

// Dimension returns the expected vector dimension.
func (g *GeminiEmbedder) Dimension() int {
	return g.dimension
}

// Request and response DTOs for Gemini batchEmbedContents API
type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiEmbedRequestItem struct {
	Model                string        `json:"model"`
	Content              geminiContent `json:"content"`
	TaskType             string        `json:"taskType,omitempty"`
	OutputDimensionality int           `json:"outputDimensionality,omitempty"`
}

type geminiBatchEmbedRequest struct {
	Requests []geminiEmbedRequestItem `json:"requests"`
}

type geminiEmbeddingValues struct {
	Values []float32 `json:"values"`
}

type geminiBatchEmbedResponse struct {
	Embeddings []geminiEmbeddingValues `json:"embeddings"`
}

// Embed generates vector embeddings for a slice of texts with batching, rate limiting, and timeout enforcement.
func (g *GeminiEmbedder) Embed(ctx context.Context, texts []string, kind EmbedKind) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	var allEmbeddings [][]float32

	// TaskType mapping
	taskType := "SEMANTIC_SIMILARITY"
	if kind == EmbedKindDocument {
		taskType = "RETRIEVAL_DOCUMENT"
	} else if kind == EmbedKindQuery {
		taskType = "RETRIEVAL_QUERY"
	}

	cleanModel := strings.TrimPrefix(g.model, "models/")
	modelResource := fmt.Sprintf("models/%s", cleanModel)

	// Process in batches up to geminiMaxBatchSize
	for i := 0; i < len(texts); i += geminiMaxBatchSize {
		end := i + geminiMaxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		batchReq := geminiBatchEmbedRequest{
			Requests: make([]geminiEmbedRequestItem, 0, len(batch)),
		}
		for _, text := range batch {
			batchReq.Requests = append(batchReq.Requests, geminiEmbedRequestItem{
				Model: modelResource,
				Content: geminiContent{
					Parts: []geminiPart{{Text: text}},
				},
				TaskType:             taskType,
				OutputDimensionality: g.dimension,
			})
		}

		batchVectors, err := g.executeBatch(ctx, cleanModel, batchReq)
		if err != nil {
			return nil, err
		}

		allEmbeddings = append(allEmbeddings, batchVectors...)
	}

	return allEmbeddings, nil
}

func (g *GeminiEmbedder) executeBatch(ctx context.Context, cleanModel string, reqPayload geminiBatchEmbedRequest) ([][]float32, error) {
	timeout := g.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Rate limiting enforcement (KF-27)
	if g.limiter != nil {
		if err := g.limiter.Wait(callCtx); err != nil {
			return nil, domain.NewTransientError("Gemini embedding rate limiter wait failed", err)
		}
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:batchEmbedContents", g.baseURL, cleanModel)

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, domain.NewPermanentError("failed to marshal Gemini embedding request", err)
	}

	sendReq := func() (*http.Response, []byte, error) {
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, nil, domain.NewPermanentError("failed to create Gemini HTTP request", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.apiKey)

		resp, err := g.httpClient.Do(req)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil, nil, domain.NewTransientError("Gemini embedding network timeout", err)
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil, nil, domain.NewTransientError("Gemini embedding context canceled/deadline exceeded", err)
			}
			return nil, nil, domain.NewTransientError("Gemini embedding network failure", err)
		}
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, domain.NewTransientError("failed to read Gemini embedding response body", err)
		}
		return resp, respBytes, nil
	}

	resp, respBytes, err := sendReq()
	if err != nil {
		return nil, err
	}

	// Honor Retry-After on 429 or 503 (KF-27)
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		retryAfter, hasRetryAfter := platform.ParseRetryAfter(resp.Header.Get("Retry-After"))
		if hasRetryAfter && retryAfter > 0 && retryAfter <= 2*time.Second {
			select {
			case <-callCtx.Done():
				return nil, domain.NewTransientError("Gemini call timed out while honoring Retry-After", callCtx.Err())
			case <-time.After(retryAfter):
			}
			// Retry once
			resp, respBytes, err = sendReq()
			if err != nil {
				return nil, err
			}
		}
	}

	// Classify HTTP response statuses
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter, hasRetryAfter := platform.ParseRetryAfter(resp.Header.Get("Retry-After"))
		msg := "Gemini rate limit exceeded (HTTP 429)"
		if hasRetryAfter {
			msg = fmt.Sprintf("Gemini rate limit exceeded (HTTP 429, Retry-After: %s)", retryAfter)
		}
		return nil, domain.NewTransientError(msg, domain.ErrRateLimited)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, domain.NewTransientError(
			fmt.Sprintf("Gemini upstream service unavailable (HTTP %d): %s", resp.StatusCode, string(respBytes)),
			domain.ErrUpstreamUnavailable,
		)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, domain.NewPermanentError(
			fmt.Sprintf("Gemini bad request error (HTTP %d): %s", resp.StatusCode, string(respBytes)),
			domain.ErrValidation,
		)
	}

	var batchResp geminiBatchEmbedResponse
	if err := json.Unmarshal(respBytes, &batchResp); err != nil {
		return nil, domain.NewPermanentError("failed to decode Gemini embedding response JSON", err)
	}

	if len(batchResp.Embeddings) != len(reqPayload.Requests) {
		return nil, domain.NewPermanentError(
			fmt.Sprintf("Gemini returned %d embeddings, expected %d", len(batchResp.Embeddings), len(reqPayload.Requests)),
			domain.ErrPermanent,
		)
	}

	// Verify embedding dimensions
	results := make([][]float32, len(batchResp.Embeddings))
	for idx, emb := range batchResp.Embeddings {
		if len(emb.Values) != g.dimension {
			return nil, domain.NewPermanentError(
				fmt.Sprintf("Gemini returned embedding with dimension %d, expected %d", len(emb.Values), g.dimension),
				domain.ErrPermanent,
			)
		}
		results[idx] = emb.Values
	}

	return results, nil
}
