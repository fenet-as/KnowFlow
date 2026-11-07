package http_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/domain"
	internalhttp "github.com/fenet-as/KnowFlow/internal/http"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMapError_StandardDomainErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "not found",
			err:            domain.NewNotFoundError("document 123 not found"),
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "validation error",
			err:            domain.NewValidationError("invalid parameter", errors.New("limit must be positive")),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "validation_failed",
		},
		{
			name:           "unauthorized",
			err:            domain.NewUnauthorizedError("missing token"),
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "payload too large",
			err:            domain.NewPayloadTooLargeError("file exceeds 1MB limit"),
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedCode:   "payload_too_large",
		},
		{
			name:           "unsupported media type",
			err:            domain.NewUnsupportedMediaTypeError("only .txt and .md allowed"),
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedCode:   "unsupported_media_type",
		},
		{
			name:           "rate limited",
			err:            domain.NewRateLimitedError("too many requests"),
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "rate_limited",
		},
		{
			name:           "upstream error",
			err:            domain.NewUpstreamError("gemini call failed", errors.New("500 from provider")),
			expectedStatus: http.StatusBadGateway,
			expectedCode:   "upstream_error",
		},
		{
			name:           "upstream unavailable",
			err:            domain.NewUpstreamUnavailableError("gemini timed out", errors.New("timeout")),
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   "upstream_unavailable",
		},
		{
			name:           "internal error",
			err:            domain.NewInternalError("unexpected database crash", errors.New("driver connection lost")),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "internal_error",
		},
		{
			name:           "untyped raw error (must be shielded as generic 500)",
			err:            errors.New("SELECT * FROM secret_table: syntax error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "internal_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, env := internalhttp.MapError(tc.err, "req-12345")
			if status != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, status)
			}
			if env.Error.Code != tc.expectedCode {
				t.Errorf("expected code %q, got %q", tc.expectedCode, env.Error.Code)
			}
			if env.Error.RequestID != "req-12345" {
				t.Errorf("expected request_id req-12345, got %s", env.Error.RequestID)
			}

			// Ensure raw sensitive error message is not leaked on untyped errors
			if tc.name == "untyped raw error (must be shielded as generic 500)" {
				if env.Error.Message != "An internal server error occurred. Please try again later." {
					t.Errorf("sensitive message leaked: %s", env.Error.Message)
				}
			}
		})
	}
}

func TestRespondWithError(t *testing.T) {
	r := gin.New()
	r.GET("/trigger-error", func(c *gin.Context) {
		internalhttp.RespondWithError(c, domain.NewNotFoundError("document missing"))
	})

	req, _ := http.NewRequest(http.MethodGet, "/trigger-error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var resp internalhttp.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}

	if resp.Error.Code != "not_found" {
		t.Errorf("expected not_found, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "document missing" {
		t.Errorf("expected 'document missing', got %s", resp.Error.Message)
	}
}

