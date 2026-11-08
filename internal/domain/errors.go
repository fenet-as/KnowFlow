package domain

import (
	"errors"
	"fmt"
)

// Standard domain error codes matching KnowFlow API specification.
type ErrorCode string

const (
	CodeUnauthorized         ErrorCode = "unauthorized"
	CodeValidationFailed     ErrorCode = "validation_failed"
	CodeNotFound             ErrorCode = "not_found"
	CodePayloadTooLarge      ErrorCode = "payload_too_large"
	CodeUnsupportedMediaType ErrorCode = "unsupported_media_type"
	CodeRateLimited          ErrorCode = "rate_limited"
	CodeUpstreamError        ErrorCode = "upstream_error"
	CodeUpstreamUnavailable  ErrorCode = "upstream_unavailable"
	CodeInternalError        ErrorCode = "internal_error"
)

// Base sentinel errors for standard errors.Is checks.
var (
	ErrNotFound             = errors.New("not found")
	ErrValidation           = errors.New("validation failed")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrPayloadTooLarge      = errors.New("payload too large")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrRateLimited          = errors.New("rate limited")
	ErrUpstream             = errors.New("upstream error")
	ErrUpstreamUnavailable  = errors.New("upstream unavailable")
	ErrInternal             = errors.New("internal server error")
	ErrTransient            = errors.New("transient error")
	ErrPermanent            = errors.New("permanent error")
)

// AppError is the canonical typed domain error used throughout the service layer.
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates an AppError for missing or cross-tenant resources.
func NewNotFoundError(message string) *AppError {
	return &AppError{
		Code:    CodeNotFound,
		Message: message,
		Err:     ErrNotFound,
	}
}

// NewValidationError creates an AppError for invalid request inputs or IDs.
func NewValidationError(message string, err error) *AppError {
	return &AppError{
		Code:    CodeValidationFailed,
		Message: message,
		Err:     errors.Join(ErrValidation, err),
	}
}

// NewUnauthorizedError creates an AppError for authentication failures.
func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Code:    CodeUnauthorized,
		Message: message,
		Err:     ErrUnauthorized,
	}
}

// NewPayloadTooLargeError creates an AppError for oversized uploads.
func NewPayloadTooLargeError(message string) *AppError {
	return &AppError{
		Code:    CodePayloadTooLarge,
		Message: message,
		Err:     ErrPayloadTooLarge,
	}
}

// NewUnsupportedMediaTypeError creates an AppError for invalid file extensions/mime types.
func NewUnsupportedMediaTypeError(message string) *AppError {
	return &AppError{
		Code:    CodeUnsupportedMediaType,
		Message: message,
		Err:     ErrUnsupportedMediaType,
	}
}

// NewRateLimitedError creates an AppError for throttled upstream or client requests.
func NewRateLimitedError(message string) *AppError {
	return &AppError{
		Code:    CodeRateLimited,
		Message: message,
		Err:     ErrRateLimited,
	}
}

// NewUpstreamError creates an AppError for failures from external providers like Gemini.
func NewUpstreamError(message string, err error) *AppError {
	return &AppError{
		Code:    CodeUpstreamError,
		Message: message,
		Err:     errors.Join(ErrUpstream, err),
	}
}

// NewUpstreamUnavailableError creates an AppError for external provider timeouts or outages.
func NewUpstreamUnavailableError(message string, err error) *AppError {
	return &AppError{
		Code:    CodeUpstreamUnavailable,
		Message: message,
		Err:     errors.Join(ErrUpstreamUnavailable, err),
	}
}

// NewInternalError creates an AppError for unexpected failures.
func NewInternalError(message string, err error) *AppError {
	return &AppError{
		Code:    CodeInternalError,
		Message: message,
		Err:     errors.Join(ErrInternal, err),
	}
}

// NewTransientError creates an AppError classified as transient (retryable with backoff).
func NewTransientError(message string, err error) *AppError {
	return &AppError{
		Code:    CodeUpstreamUnavailable,
		Message: message,
		Err:     errors.Join(ErrTransient, err),
	}
}

// NewPermanentError creates an AppError classified as permanent (non-retryable, fails fast).
func NewPermanentError(message string, err error) *AppError {
	return &AppError{
		Code:    CodeInternalError,
		Message: message,
		Err:     errors.Join(ErrPermanent, err),
	}
}

// IsTransient checks if the error or any wrapped error is ErrTransient.
func IsTransient(err error) bool {
	return errors.Is(err, ErrTransient)
}

// IsPermanent checks if the error or any wrapped error is ErrPermanent.
func IsPermanent(err error) bool {
	return errors.Is(err, ErrPermanent)
}
