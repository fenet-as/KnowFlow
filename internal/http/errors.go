package http

import (
	"errors"
	"net/http"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/http/middleware"
	"github.com/gin-gonic/gin"
)

// ErrorResponse represents the top-level error envelope required by KnowFlow API contract.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the structured fields of the error.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// MapError translates a domain error into an HTTP status code and client-safe ErrorDetail.
// It centralizes all domain-to-HTTP mapping in one single location.
func MapError(err error, requestID string) (int, ErrorResponse) {
	if err == nil {
		return http.StatusOK, ErrorResponse{}
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		status := mapCodeToStatus(appErr.Code)
		return status, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(appErr.Code),
				Message:   appErr.Message,
				RequestID: requestID,
			},
		}
	}

	// Check sentinel errors
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeNotFound),
				Message:   "Resource not found.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrValidation):
		return http.StatusBadRequest, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeValidationFailed),
				Message:   "Invalid request parameters or payload.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeUnauthorized),
				Message:   "Authentication required or invalid credentials.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodePayloadTooLarge),
				Message:   "Uploaded payload exceeds the maximum allowed size.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrUnsupportedMediaType):
		return http.StatusUnsupportedMediaType, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeUnsupportedMediaType),
				Message:   "Unsupported media type. Only .txt and .md files are supported.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrRateLimited):
		return http.StatusTooManyRequests, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeRateLimited),
				Message:   "Rate limit exceeded. Please retry after some time.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrUpstream):
		return http.StatusBadGateway, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeUpstreamError),
				Message:   "An external service failure occurred.",
				RequestID: requestID,
			},
		}
	case errors.Is(err, domain.ErrUpstreamUnavailable):
		return http.StatusServiceUnavailable, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeUpstreamUnavailable),
				Message:   "An external service is temporarily unavailable.",
				RequestID: requestID,
			},
		}
	default:
		// Default to generic 500 without leaking internal details or SQL errors
		return http.StatusInternalServerError, ErrorResponse{
			Error: ErrorDetail{
				Code:      string(domain.CodeInternalError),
				Message:   "An internal server error occurred. Please try again later.",
				RequestID: requestID,
			},
		}
	}
}

func mapCodeToStatus(code domain.ErrorCode) int {
	switch code {
	case domain.CodeUnauthorized:
		return http.StatusUnauthorized
	case domain.CodeValidationFailed:
		return http.StatusBadRequest
	case domain.CodeNotFound:
		return http.StatusNotFound
	case domain.CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case domain.CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case domain.CodeRateLimited:
		return http.StatusTooManyRequests
	case domain.CodeUpstreamError:
		return http.StatusBadGateway
	case domain.CodeUpstreamUnavailable:
		return http.StatusServiceUnavailable
	case domain.CodeInternalError:
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}

// RespondWithError attaches the error to Gin context and sends the standard JSON error envelope.
func RespondWithError(c *gin.Context, err error) {
	reqID := middleware.GetRequestID(c.Request.Context())
	status, envelope := MapError(err, reqID)

	_ = c.Error(err)
	c.AbortWithStatusJSON(status, envelope)
}

