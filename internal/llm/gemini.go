package llm

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
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

// GeminiLLMConfig configures the Gemini generative client.
type GeminiLLMConfig struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// GeminiLLM implements LLM using Google Gemini's REST API.
type GeminiLLM struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// NewGeminiLLM creates an initialized GeminiLLM.
func NewGeminiLLM(cfg GeminiLLMConfig) (*GeminiLLM, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, domain.NewPermanentError("gemini API key is required", domain.ErrValidation)
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gemini-1.5-flash"
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}

	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}

	return &GeminiLLM{
		apiKey:     cfg.APIKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: client,
		timeout:    cfg.Timeout,
	}, nil
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	Temperature float32 `json:"temperature"`
}

type geminiGenerateRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiGenerateResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

// Generate sends the prompt to the Gemini model and returns the generated text.
func (g *GeminiLLM) Generate(ctx context.Context, prompt string) (string, error) {
	cleanModel := strings.TrimPrefix(g.model, "models/")
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, cleanModel)

	reqPayload := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: geminiGenConfig{
			Temperature: 0.2, // Low temperature for deterministic, factual retrieval
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", domain.NewPermanentError("failed to marshal Gemini generation request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", domain.NewPermanentError("failed to create Gemini generation HTTP request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "", domain.NewTransientError("Gemini generation network timeout", err)
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", domain.NewTransientError("Gemini generation context deadline exceeded", err)
		}
		return "", domain.NewTransientError("Gemini generation network failure", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", domain.NewTransientError("failed to read Gemini generation response body", err)
	}

	// Classify HTTP response statuses
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", domain.NewTransientError("Gemini generation rate limit exceeded (HTTP 429)", domain.ErrRateLimited)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return "", domain.NewTransientError(
			fmt.Sprintf("Gemini upstream service unavailable (HTTP %d): %s", resp.StatusCode, string(respBytes)),
			domain.ErrUpstreamUnavailable,
		)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", domain.NewPermanentError(
			fmt.Sprintf("Gemini generation API rejected request (HTTP %d): %s", resp.StatusCode, string(respBytes)),
			domain.ErrPermanent,
		)
	}

	var genResp geminiGenerateResponse
	if err := json.Unmarshal(respBytes, &genResp); err != nil {
		return "", domain.NewPermanentError("failed to decode Gemini generation response JSON", err)
	}

	if len(genResp.Candidates) == 0 || len(genResp.Candidates[0].Content.Parts) == 0 {
		return "", domain.NewPermanentError("Gemini returned zero response candidates", domain.ErrUpstream)
	}

	return genResp.Candidates[0].Content.Parts[0].Text, nil
}
