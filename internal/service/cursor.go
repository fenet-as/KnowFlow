package service

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
)

// EncodeCursor produces an opaque base64 cursor representing (created_at, id).
func EncodeCursor(t time.Time, id uuid.UUID) string {
	payload := fmt.Sprintf("%s,%s", t.UTC().Format(time.RFC3339Nano), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// DecodeCursor decodes an opaque cursor into (created_at, id).
// Returns nil pointers when cursor is empty. Returns a domain validation error if malformed.
func DecodeCursor(cursor string) (*time.Time, *uuid.UUID, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return nil, nil, nil
	}

	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, nil, domain.NewValidationError("invalid cursor: unable to decode base64", err)
		}
	}

	parts := strings.Split(string(data), ",")
	if len(parts) != 2 {
		return nil, nil, domain.NewValidationError("invalid cursor: malformed payload", nil)
	}

	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		t, err = time.Parse(time.RFC3339, parts[0])
		if err != nil {
			return nil, nil, domain.NewValidationError("invalid cursor: invalid timestamp format", err)
		}
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, nil, domain.NewValidationError("invalid cursor: invalid UUID", err)
	}

	utcTime := t.UTC()
	return &utcTime, &id, nil
}
