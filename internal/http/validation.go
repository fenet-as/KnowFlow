package http

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
	MinPageLimit     = 1
)

// ParseUUID parses a string into a UUID or returns a domain validation error.
func ParseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, domain.NewValidationError(fmt.Sprintf("invalid UUID parameter '%s'", s), err)
	}
	return id, nil
}

// RequireUUIDParam extracts a URL path parameter and validates that it is a valid UUID.
// If invalid, it responds immediately with HTTP 400 and returns false.
func RequireUUIDParam(c *gin.Context, paramName string) (uuid.UUID, bool) {
	val := c.Param(paramName)
	id, err := ParseUUID(val)
	if err != nil {
		RespondWithError(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// ParsePagination parses and validates limit and cursor query parameters.
func ParsePagination(limitStr, cursorStr string) (int, string, error) {
	limit := DefaultPageLimit

	limitStr = strings.TrimSpace(limitStr)
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			return 0, "", domain.NewValidationError("query parameter 'limit' must be an integer", err)
		}
		if parsed < MinPageLimit || parsed > MaxPageLimit {
			return 0, "", domain.NewValidationError(fmt.Sprintf("query parameter 'limit' must be between %d and %d", MinPageLimit, MaxPageLimit), nil)
		}
		limit = parsed
	}

	return limit, strings.TrimSpace(cursorStr), nil
}

// RequirePagination extracts and validates standard limit and cursor query parameters.
func RequirePagination(c *gin.Context) (int, string, bool) {
	limitStr := c.Query("limit")
	cursorStr := c.Query("cursor")

	limit, cursor, err := ParsePagination(limitStr, cursorStr)
	if err != nil {
		RespondWithError(c, err)
		return 0, "", false
	}
	return limit, cursor, true
}

