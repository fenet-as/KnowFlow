package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	internalhttp "github.com/fenet-as/KnowFlow/internal/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestParseUUID(t *testing.T) {
	t.Run("valid uuid", func(t *testing.T) {
		raw := uuid.NewString()
		parsed, err := internalhttp.ParseUUID(raw)
		if err != nil {
			t.Fatalf("expected valid UUID, got: %v", err)
		}
		if parsed.String() != raw {
			t.Errorf("expected %s, got %s", raw, parsed.String())
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		_, err := internalhttp.ParseUUID("not-a-uuid")
		if err == nil {
			t.Fatal("expected error on invalid UUID string, got nil")
		}
	})

	t.Run("empty uuid", func(t *testing.T) {
		_, err := internalhttp.ParseUUID("")
		if err == nil {
			t.Fatal("expected error on empty UUID string, got nil")
		}
	})
}

func TestRequireUUIDParam(t *testing.T) {
	r := gin.New()
	r.GET("/documents/:id", func(c *gin.Context) {
		id, ok := internalhttp.RequireUUIDParam(c, "id")
		if !ok {
			return
		}
		c.String(http.StatusOK, id.String())
	})

	t.Run("valid path param", func(t *testing.T) {
		validID := uuid.NewString()
		req, _ := http.NewRequest(http.MethodGet, "/documents/"+validID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != validID {
			t.Errorf("expected %s, got %s", validID, w.Body.String())
		}
	})

	t.Run("invalid path param returns 400 validation error", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/documents/123-bad-id", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 BadRequest, got %d", w.Code)
		}
	})
}

func TestParsePagination(t *testing.T) {
	t.Run("default limit and empty cursor", func(t *testing.T) {
		limit, cursor, err := internalhttp.ParsePagination("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if limit != 20 {
			t.Errorf("expected default limit 20, got %d", limit)
		}
		if cursor != "" {
			t.Errorf("expected empty cursor, got %s", cursor)
		}
	})

	t.Run("valid custom limit and cursor", func(t *testing.T) {
		limit, cursor, err := internalhttp.ParsePagination("50", "cursor_token_xyz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if limit != 50 {
			t.Errorf("expected limit 50, got %d", limit)
		}
		if cursor != "cursor_token_xyz" {
			t.Errorf("expected cursor_token_xyz, got %s", cursor)
		}
	})

	t.Run("limit exceeds maximum 100", func(t *testing.T) {
		_, _, err := internalhttp.ParsePagination("200", "")
		if err == nil {
			t.Fatal("expected error when limit > 100, got nil")
		}
	})

	t.Run("limit below minimum 1", func(t *testing.T) {
		_, _, err := internalhttp.ParsePagination("0", "")
		if err == nil {
			t.Fatal("expected error when limit < 1, got nil")
		}
	})

	t.Run("non-numeric limit", func(t *testing.T) {
		_, _, err := internalhttp.ParsePagination("abc", "")
		if err == nil {
			t.Fatal("expected error when limit is non-numeric, got nil")
		}
	})
}
