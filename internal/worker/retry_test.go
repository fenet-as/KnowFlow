package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/worker"
	"github.com/stretchr/testify/assert"
)

type dummyTimeoutNetError struct{}

func (e *dummyTimeoutNetError) Error() string   { return "i/o timeout" }
func (e *dummyTimeoutNetError) Timeout() bool   { return true }
func (e *dummyTimeoutNetError) Temporary() bool { return true }

func TestClassifyError_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		isTransient bool
	}{
		{
			name:        "nil error",
			err:         nil,
			isTransient: false,
		},
		{
			name:        "HTTP 429 rate limit error",
			err:         errors.New("Gemini API error (HTTP 429): Resource exhausted"),
			isTransient: true,
		},
		{
			name:        "Rate limit keyword",
			err:         errors.New("rate limit reached for current quota"),
			isTransient: true,
		},
		{
			name:        "HTTP 500 internal server error",
			err:         errors.New("HTTP 500 Internal Server Error"),
			isTransient: true,
		},
		{
			name:        "HTTP 502 bad gateway",
			err:         errors.New("HTTP 502 Bad Gateway"),
			isTransient: true,
		},
		{
			name:        "HTTP 503 service unavailable",
			err:         errors.New("HTTP 503 Service Unavailable"),
			isTransient: true,
		},
		{
			name:        "HTTP 504 gateway timeout",
			err:         errors.New("HTTP 504 Gateway Timeout"),
			isTransient: true,
		},
		{
			name:        "context.DeadlineExceeded",
			err:         context.DeadlineExceeded,
			isTransient: true,
		},
		{
			name:        "net.Error timeout",
			err:         &dummyTimeoutNetError{},
			isTransient: true,
		},
		{
			name:        "connection reset by peer",
			err:         errors.New("read tcp: connection reset by peer"),
			isTransient: true,
		},
		{
			name:        "connection refused",
			err:         errors.New("dial tcp: connection refused"),
			isTransient: true,
		},
		{
			name:        "broken pipe",
			err:         errors.New("write: broken pipe"),
			isTransient: true,
		},
		{
			name:        "domain.ErrRateLimited",
			err:         domain.NewRateLimitedError("too many requests"),
			isTransient: true,
		},
		{
			name:        "domain.ErrUpstreamUnavailable",
			err:         domain.NewUpstreamUnavailableError("upstream provider down", nil),
			isTransient: true,
		},
		{
			name:        "domain.NewTransientError",
			err:         domain.NewTransientError("transient embedding hiccup", nil),
			isTransient: true,
		},
		{
			name:        "HTTP 400 bad request",
			err:         errors.New("HTTP 400 Bad Request: invalid argument"),
			isTransient: false,
		},
		{
			name:        "HTTP 401 unauthorized",
			err:         errors.New("HTTP 401 Unauthorized"),
			isTransient: false,
		},
		{
			name:        "HTTP 403 forbidden",
			err:         errors.New("HTTP 403 Forbidden: API key disabled"),
			isTransient: false,
		},
		{
			name:        "HTTP 404 not found",
			err:         errors.New("HTTP 404 Not Found"),
			isTransient: false,
		},
		{
			name:        "domain validation error",
			err:         domain.NewValidationError("document empty", nil),
			isTransient: false,
		},
		{
			name:        "domain not found error",
			err:         domain.NewNotFoundError("document not found"),
			isTransient: false,
		},
		{
			name:        "domain permanent error",
			err:         domain.NewPermanentError("dimension mismatch: got 512, want 768", nil),
			isTransient: false,
		},
		{
			name:        "general syntax or parse error",
			err:         errors.New("syntax error: unexpected token"),
			isTransient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worker.ClassifyError(tt.err)
			assert.Equal(t, tt.isTransient, got, "expected ClassifyError(%v) = %v, got %v", tt.err, tt.isTransient, got)
		})
	}
}

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := worker.RetryPolicy{
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		MaxAttempts: 5,
	}

	// Attempt 1: ~100ms
	d1 := policy.CalculateDelay(1)
	assert.InDelta(t, 100*time.Millisecond, d1, float64(30*time.Millisecond))

	// Attempt 2: ~200ms
	d2 := policy.CalculateDelay(2)
	assert.InDelta(t, 200*time.Millisecond, d2, float64(50*time.Millisecond))

	// Attempt 3: ~400ms
	d3 := policy.CalculateDelay(3)
	assert.InDelta(t, 400*time.Millisecond, d3, float64(100*time.Millisecond))

	// Attempt 4: ~800ms
	d4 := policy.CalculateDelay(4)
	assert.InDelta(t, 800*time.Millisecond, d4, float64(200*time.Millisecond))

	// Attempt 10: should clamp to MaxDelay (2s)
	d10 := policy.CalculateDelay(10)
	assert.LessOrEqual(t, d10, 2500*time.Millisecond)
}
