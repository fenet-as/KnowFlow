package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRAGService struct {
	askFn func(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID) (*domain.Answer, error)
}

func (m *mockRAGService) Ask(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID) (*domain.Answer, error) {
	if m.askFn != nil {
		return m.askFn(ctx, ownerID, question, documentIDs)
	}
	return &domain.Answer{
		Answer:    "Default answer [1].",
		Grounded:  true,
		Citations: []domain.Citation{},
	}, nil
}

func (m *mockRAGService) AskWithHistory(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID, priorTurns []llm.ConversationTurn) (*domain.Answer, error) {
	return m.Ask(ctx, ownerID, question, documentIDs)
}

func setupTestRAGRouter(svc *mockRAGService, ownerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewRAGHandler(svc)

	authMiddleware := func(c *gin.Context) {
		if ownerID != "" {
			c.Request = c.Request.WithContext(auth.WithOwnerID(c.Request.Context(), ownerID))
		}
		c.Next()
	}

	api := r.Group("/api/v1")
	api.Use(authMiddleware)
	{
		api.POST("/ask", handler.Ask)
	}

	return r
}

func TestRAGHandler_Ask(t *testing.T) {
	ownerID := "user_ask_test"
	docID := uuid.New()
	chunkID := uuid.New()

	t.Run("successful grounded answer returns 200 with citations", func(t *testing.T) {
		mockSvc := &mockRAGService{
			askFn: func(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID) (*domain.Answer, error) {
				assert.Equal(t, "user_ask_test", ownerID)
				assert.Equal(t, "How many PTO days?", question)
				return &domain.Answer{
					Answer:   "You get 15 days of PTO [1].",
					Grounded: true,
					Citations: []domain.Citation{
						{
							Marker:       1,
							ChunkID:      chunkID,
							DocumentID:   docID,
							DocumentName: "handbook.md",
							Snippet:      "Employees get 15 days...",
						},
					},
				}, nil
			},
		}

		router := setupTestRAGRouter(mockSvc, ownerID)
		payload := `{"question": "How many PTO days?"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/ask", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp domain.Answer
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Grounded)
		assert.Equal(t, "You get 15 days of PTO [1].", resp.Answer)
		require.Len(t, resp.Citations, 1)
		assert.Equal(t, chunkID, resp.Citations[0].ChunkID)
	})

	t.Run("no relevant context returns 200 with fixed message and empty citations array", func(t *testing.T) {
		mockSvc := &mockRAGService{
			askFn: func(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID) (*domain.Answer, error) {
				return &domain.Answer{
					Answer:    "I could not find this in your documents.",
					Grounded:  false,
					Citations: []domain.Citation{},
				}, nil
			},
		}

		router := setupTestRAGRouter(mockSvc, ownerID)
		payload := `{"question": "What is the speed of light?"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/ask", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp domain.Answer
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.False(t, resp.Grounded)
		assert.Equal(t, "I could not find this in your documents.", resp.Answer)
		assert.Empty(t, resp.Citations)
	})

	t.Run("empty question returns 400 validation error", func(t *testing.T) {
		router := setupTestRAGRouter(&mockRAGService{}, ownerID)
		payload := `{"question": "   "}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/ask", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		assert.Equal(t, string(domain.CodeValidationFailed), errResp.Error.Code)
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		router := setupTestRAGRouter(&mockRAGService{}, "") // No owner ID
		payload := `{"question": "Valid question?"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/ask", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("upstream LLM failure returns 503 Service Unavailable", func(t *testing.T) {
		mockSvc := &mockRAGService{
			askFn: func(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID) (*domain.Answer, error) {
				return nil, domain.NewUpstreamUnavailableError("Gemini unavailable", nil)
			},
		}

		router := setupTestRAGRouter(mockSvc, ownerID)
		payload := `{"question": "Valid question?"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/ask", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}
