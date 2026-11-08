package service

import (
	"context"
	"errors"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/embed"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChunkRepo struct {
	bulkInsertFn       func(ctx context.Context, chunks []*domain.Chunk) error
	deleteByDocFn      func(ctx context.Context, ownerID string, documentID uuid.UUID) error
	getByIDFn          func(ctx context.Context, ownerID string, id uuid.UUID) (*domain.ChunkDetail, error)
	similaritySearchFn func(ctx context.Context, ownerID string, embedding []float32, documentIDs []uuid.UUID, limit int) ([]domain.ChunkMatch, error)
}

func (m *mockChunkRepo) BulkInsert(ctx context.Context, chunks []*domain.Chunk) error {
	if m.bulkInsertFn != nil {
		return m.bulkInsertFn(ctx, chunks)
	}
	return nil
}

func (m *mockChunkRepo) DeleteByDocument(ctx context.Context, ownerID string, documentID uuid.UUID) error {
	if m.deleteByDocFn != nil {
		return m.deleteByDocFn(ctx, ownerID, documentID)
	}
	return nil
}

func (m *mockChunkRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.ChunkDetail, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, ownerID, id)
	}
	return nil, domain.NewNotFoundError("Chunk not found.")
}

func (m *mockChunkRepo) SimilaritySearch(ctx context.Context, ownerID string, embedding []float32, documentIDs []uuid.UUID, limit int) ([]domain.ChunkMatch, error) {
	if m.similaritySearchFn != nil {
		return m.similaritySearchFn(ctx, ownerID, embedding, documentIDs, limit)
	}
	return nil, nil
}

func TestRetrievalService_RelevanceGate(t *testing.T) {
	fakeEmbedder := embed.NewFakeEmbedder(768)
	ctx := context.Background()
	ownerID := "user_test_retrieval"

	t.Run("top score exceeds threshold: returns relevant context", func(t *testing.T) {
		mockRepo := &mockChunkRepo{
			similaritySearchFn: func(ctx context.Context, ownerID string, embedding []float32, documentIDs []uuid.UUID, limit int) ([]domain.ChunkMatch, error) {
				assert.Equal(t, 6, limit)
				return []domain.ChunkMatch{
					{
						ID:           uuid.New(),
						DocumentName: "policy.md",
						Content:      "Employees get 15 days of paid time off.",
						Score:        0.85,
					},
					{
						ID:           uuid.New(),
						DocumentName: "policy.md",
						Content:      "Unused days carry over.",
						Score:        0.72,
					},
				}, nil
			},
		}

		retrievalSvc := NewRetrievalService(fakeEmbedder, mockRepo, RetrievalConfig{
			TopK:     6,
			MinScore: 0.65,
		}, nil)

		res, err := retrievalSvc.Retrieve(ctx, ownerID, "How many PTO days?", nil)
		require.NoError(t, err)
		assert.True(t, res.HasRelevantContext)
		assert.Len(t, res.Matches, 2)
		assert.Equal(t, 0.85, res.TopScore)
	})

	t.Run("top score below threshold: returns no relevant context", func(t *testing.T) {
		mockRepo := &mockChunkRepo{
			similaritySearchFn: func(ctx context.Context, ownerID string, embedding []float32, documentIDs []uuid.UUID, limit int) ([]domain.ChunkMatch, error) {
				return []domain.ChunkMatch{
					{
						ID:           uuid.New(),
						DocumentName: "unrelated.md",
						Content:      "The cafeteria serves pizza on Friday.",
						Score:        0.42, // Under 0.65 threshold
					},
				}, nil
			},
		}

		retrievalSvc := NewRetrievalService(fakeEmbedder, mockRepo, RetrievalConfig{
			TopK:     6,
			MinScore: 0.65,
		}, nil)

		res, err := retrievalSvc.Retrieve(ctx, ownerID, "What is quantum computing?", nil)
		require.NoError(t, err)
		assert.False(t, res.HasRelevantContext)
		assert.Empty(t, res.Matches)
		assert.Equal(t, 0.42, res.TopScore)
	})

	t.Run("zero matches returns no relevant context", func(t *testing.T) {
		mockRepo := &mockChunkRepo{
			similaritySearchFn: func(ctx context.Context, ownerID string, embedding []float32, documentIDs []uuid.UUID, limit int) ([]domain.ChunkMatch, error) {
				return []domain.ChunkMatch{}, nil
			},
		}

		retrievalSvc := NewRetrievalService(fakeEmbedder, mockRepo, RetrievalConfig{
			TopK:     6,
			MinScore: 0.65,
		}, nil)

		res, err := retrievalSvc.Retrieve(ctx, ownerID, "Any question", nil)
		require.NoError(t, err)
		assert.False(t, res.HasRelevantContext)
		assert.Empty(t, res.Matches)
		assert.Equal(t, 0.0, res.TopScore)
	})

	t.Run("empty query returns 400 validation error", func(t *testing.T) {
		retrievalSvc := NewRetrievalService(fakeEmbedder, &mockChunkRepo{}, RetrievalConfig{}, nil)
		_, err := retrievalSvc.Retrieve(ctx, ownerID, "   ", nil)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeValidationFailed, appErr.Code)
	})

	t.Run("missing owner returns 401 unauthorized", func(t *testing.T) {
		retrievalSvc := NewRetrievalService(fakeEmbedder, &mockChunkRepo{}, RetrievalConfig{}, nil)
		_, err := retrievalSvc.Retrieve(ctx, "", "query", nil)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeUnauthorized, appErr.Code)
	})

	t.Run("embedder error propagates cleanly", func(t *testing.T) {
		errEmbed := domain.NewTransientError("embedder rate limited", domain.ErrRateLimited)
		fakeErrEmbedder := embed.NewFakeEmbedder(768)
		fakeErrEmbedder.SetError(errEmbed)

		retrievalSvc := NewRetrievalService(fakeErrEmbedder, &mockChunkRepo{}, RetrievalConfig{}, nil)
		_, err := retrievalSvc.Retrieve(ctx, ownerID, "some query", nil)
		assert.Error(t, err)
		assert.True(t, domain.IsTransient(err))
	})
}
