package worker

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
)

// RetryPolicy defines backoff timing parameters and maximum attempt limits for async tasks.
type RetryPolicy struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	MaxAttempts int
}

// DefaultRetryPolicy returns standard production defaults (1s base, 1m max, 5 attempts).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		BaseDelay:   1 * time.Second,
		MaxDelay:    1 * time.Minute,
		MaxAttempts: 5,
	}
}

// CalculateDelay calculates exponential backoff with +/- 20% randomized jitter:
// delay = min(MaxDelay, BaseDelay * 2^(attempts-1)) +/- jitter
func (p RetryPolicy) CalculateDelay(attempts int) time.Duration {
	if attempts <= 0 {
		attempts = 1
	}
	base := p.BaseDelay
	if base <= 0 {
		base = 1 * time.Second
	}
	maxD := p.MaxDelay
	if maxD <= 0 {
		maxD = 1 * time.Minute
	}

	shift := attempts - 1
	if shift > 10 {
		shift = 10 // Prevent integer overflow
	}

	multiplier := time.Duration(1 << shift)
	delay := base * multiplier
	if delay > maxD {
		delay = maxD
	}

	// Apply +/- 20% pseudo-random jitter
	jitterMagnitude := float64(delay) * 0.2
	if jitterMagnitude > 0 {
		jitterDelta := time.Duration((rand.Float64()*2 - 1) * jitterMagnitude)
		delay += jitterDelta
	}

	if delay < base {
		delay = base
	}
	return delay
}

// ClassifyError returns true if err represents a transient, retryable failure.
// Transient errors include HTTP 429, 5xx, timeouts, network interruptions, and domain transient sentinels.
// Permanent errors (validation, auth, 4xx, schema mismatch) return false.
func ClassifyError(err error) bool {
	if err == nil {
		return false
	}

	// 1. Explicit domain classification checks
	if domain.IsPermanent(err) {
		return false
	}
	if domain.IsTransient(err) {
		return true
	}

	// Check domain base sentinels
	if errors.Is(err, domain.ErrRateLimited) ||
		errors.Is(err, domain.ErrUpstreamUnavailable) ||
		errors.Is(err, domain.ErrTransient) {
		return true
	}

	if errors.Is(err, domain.ErrValidation) ||
		errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrUnauthorized) ||
		errors.Is(err, domain.ErrPayloadTooLarge) ||
		errors.Is(err, domain.ErrUnsupportedMediaType) ||
		errors.Is(err, domain.ErrPermanent) {
		return false
	}

	// 2. Standard library context, IO, and network errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// 3. String-based matching for external API responses & network sockets
	msg := strings.ToLower(err.Error())

	// HTTP status codes and transient phrases
	transientPatterns := []string{
		"429",
		"too many requests",
		"rate limit",
		"quota exceeded",
		"resource exhausted",
		"500 internal server",
		"502 bad gateway",
		"503 service unavailable",
		"504 gateway timeout",
		"server unavailable",
		"connection refused",
		"connection reset",
		"broken pipe",
		"network is unreachable",
		"temporary failure",
		"i/o timeout",
		"unexpected eof",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	// By default, unknown internal or syntax errors are treated as permanent to avoid infinite loops
	return false
}
