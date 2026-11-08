package platform

import (
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter parses the HTTP Retry-After response header into a time.Duration.
// It supports integer seconds, float seconds, and standard HTTP-date formats (RFC1123, RFC850, ANSIC).
// Returns (0, false) if the header is empty or invalid.
func ParseRetryAfter(header string) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}

	// 1. Integer seconds (e.g., "5", "120")
	if secs, err := strconv.Atoi(header); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}

	// 2. Float seconds (e.g., "1.5")
	if secs, err := strconv.ParseFloat(header, 64); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs * float64(time.Second)), true
	}

	// 3. HTTP-date formats
	layouts := []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC850,
		time.ANSIC,
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, header); err == nil {
			delay := time.Until(t)
			if delay < 0 {
				return 0, true
			}
			return delay, true
		}
	}

	return 0, false
}
