package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChunkRepo struct {
	getByIDFn func(ctx context.Context, ownerID string, id uuid.UUID) (*domain.ChunkDetail, error)
}

func (m *mockChunkRepo) BulkInsert(ctx context.Context, chunks []*domain.Chunk) error {
	return nil
}

func (m *mockChunkRepo) DeleteByDocument(ctx context.Context, ownerID string, documentID uuid.UUID) error {
	return nil
}

func (m *mockChunkRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.ChunkDetail, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, ownerID, id)
	}
	return nil, domain.NewNotFoundError("Chunk not found.")
}

func (m *mockChunkRepo) SimilaritySearch(ctx context.Context, ownerID string, embedding []float32, documentIDs []uuid.UUID, limit int) ([]domain.ChunkMatch, error) {
	return nil, nil
}

func setupTestChunkRouter(repo *mockChunkRepo, ownerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewChunkHandler(repo)

	authMiddleware := func(c *gin.Context) {
		if ownerID != "" {
			c.Request = c.Request.WithContext(auth.WithOwnerID(c.Request.Context(), ownerID))
		}
		c.Next()
	}

	api := r.Group("/api/v1")
	api.Use(authMiddleware)
	{
		api.GET("/chunks/:id", handler.GetByID)
	}

	return r
}

func TestChunkHandler_GetByID_Success(t *testing.T) {
	chunkID := uuid.New()
	docID := uuid.New()
	ownerID := "user_test_chunk_1"
	heading := "Return Policy"

	mockRepo := &mockChunkRepo{
		getByIDFn: func(ctx context.Context, owner string, id uuid.UUID) (*domain.ChunkDetail, error) {
			assert.Equal(t, ownerID, owner)
			assert.Equal(t, chunkID, id)
			return &domain.ChunkDetail{
				ID:           chunkID,
				DocumentID:   docID,
				DocumentName: "policy.md",
				ChunkIndex:   2,
				Heading:      &heading,
				Content:      "Items can be returned within 30 days of purchase.",
			}, nil
		},
	}

	router := setupTestChunkRouter(mockRepo, ownerID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chunks/"+chunkID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.ChunkDetail
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, chunkID, resp.ID)
	assert.Equal(t, docID, resp.DocumentID)
	assert.Equal(t, "policy.md", resp.DocumentName)
	assert.Equal(t, 2, resp.ChunkIndex)
	require.NotNil(t, resp.Heading)
	assert.Equal(t, "Return Policy", *resp.Heading)
	assert.Equal(t, "Items can be returned within 30 days of purchase.", resp.Content)
}

func TestChunkHandler_GetByID_NotFoundOrCrossTenant(t *testing.T) {
	chunkID := uuid.New()
	ownerID := "user_test_chunk_1"

	mockRepo := &mockChunkRepo{
		getByIDFn: func(ctx context.Context, owner string, id uuid.UUID) (*domain.ChunkDetail, error) {
			return nil, domain.NewNotFoundError("Chunk not found.")
		},
	}

	router := setupTestChunkRouter(mockRepo, ownerID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chunks/"+chunkID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	errBody := errResp["error"].(map[string]any)
	assert.Equal(t, "not_found", errBody["code"])
	assert.Equal(t, "Chunk not found.", errBody["message"])
}

func TestChunkHandler_GetByID_InvalidUUID(t *testing.T) {
	ownerID := "user_test_chunk_1"
	mockRepo := &mockChunkRepo{}

	router := setupTestChunkRouter(mockRepo, ownerID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chunks/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	errBody := errResp["error"].(map[string]any)
	assert.Equal(t, "validation_failed", errBody["code"])
}

func TestChunkHandler_GetByID_Unauthorized(t *testing.T) {
	chunkID := uuid.New()
	mockRepo := &mockChunkRepo{}

	// No ownerID injected in auth middleware
	router := setupTestChunkRouter(mockRepo, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chunks/"+chunkID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
