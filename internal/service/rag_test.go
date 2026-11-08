package service

import (
	"context"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRetrievalService struct {
	retrieveFn func(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error)
}

func (m *mockRetrievalService) Retrieve(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error) {
	if m.retrieveFn != nil {
		return m.retrieveFn(ctx, ownerID, query, documentIDs)
	}
	return &RetrievalResult{HasRelevantContext: false}, nil
}

func TestRAGService_Ask(t *testing.T) {
	ctx := context.Background()
	ownerID := "user_rag_test"
	docID := uuid.New()
	chunkID := uuid.New()
	heading := "Policy Section"

	matches := []domain.ChunkMatch{
		{
			ID:           chunkID,
			DocumentID:   docID,
			DocumentName: "handbook.md",
			Heading:      &heading,
			Content:      "Employees receive 15 days of paid time off per year.",
			Score:        0.89,
		},
	}

	t.Run("successful grounded answer with citations", func(t *testing.T) {
		mockRetrieval := &mockRetrievalService{
			retrieveFn: func(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error) {
				return &RetrievalResult{
					HasRelevantContext: true,
					Matches:            matches,
					TopScore:           0.89,
					Query:              query,
				}, nil
			},
		}

		fakeLLM := llm.NewFakeLLM("Employees receive 15 days of PTO [1].")
		ragSvc := NewRAGService(mockRetrieval, fakeLLM, nil)

		ans, err := ragSvc.Ask(ctx, ownerID, "How many days off?", nil)
		require.NoError(t, err)
		assert.True(t, ans.Grounded)
		assert.Equal(t, "Employees receive 15 days of PTO [1].", ans.Answer)
		require.Len(t, ans.Citations, 1)
		assert.Equal(t, 1, ans.Citations[0].Marker)
		assert.Equal(t, chunkID, ans.Citations[0].ChunkID)
		assert.Equal(t, "handbook.md", ans.Citations[0].DocumentName)
		assert.Equal(t, 1, fakeLLM.Calls())
	})

	t.Run("no relevant context returns fixed fallback without calling LLM", func(t *testing.T) {
		mockRetrieval := &mockRetrievalService{
			retrieveFn: func(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error) {
				return &RetrievalResult{
					HasRelevantContext: false,
					Matches:            []domain.ChunkMatch{},
					TopScore:           0.30,
					Query:              query,
				}, nil
			},
		}

		fakeLLM := llm.NewFakeLLM("Should not be called")
		ragSvc := NewRAGService(mockRetrieval, fakeLLM, nil)

		ans, err := ragSvc.Ask(ctx, ownerID, "Unrelated question", nil)
		require.NoError(t, err)
		assert.False(t, ans.Grounded)
		assert.Equal(t, FixedNoRelevantContextMessage, ans.Answer)
		assert.Empty(t, ans.Citations)
		assert.Equal(t, 0, fakeLLM.Calls(), "LLM must NOT be called when relevance gate rejects")
	})

	t.Run("LLM answer lacking valid citations falls back to fixed answer", func(t *testing.T) {
		mockRetrieval := &mockRetrievalService{
			retrieveFn: func(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error) {
				return &RetrievalResult{
					HasRelevantContext: true,
					Matches:            matches,
					TopScore:           0.89,
				}, nil
			},
		}

		// LLM generates text without any [1] marker
		fakeLLM := llm.NewFakeLLM("I am an AI and I think you get some days.")
		ragSvc := NewRAGService(mockRetrieval, fakeLLM, nil)

		ans, err := ragSvc.Ask(ctx, ownerID, "How many days off?", nil)
		require.NoError(t, err)
		assert.False(t, ans.Grounded)
		assert.Equal(t, FixedNoRelevantContextMessage, ans.Answer)
		assert.Empty(t, ans.Citations)
	})

	t.Run("empty question returns validation error", func(t *testing.T) {
		ragSvc := NewRAGService(&mockRetrievalService{}, llm.NewFakeLLM(""), nil)
		_, err := ragSvc.Ask(ctx, ownerID, "   ", nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("LLM error propagates", func(t *testing.T) {
		mockRetrieval := &mockRetrievalService{
			retrieveFn: func(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error) {
				return &RetrievalResult{
					HasRelevantContext: true,
					Matches:            matches,
				}, nil
			},
		}

		fakeLLM := llm.NewFakeLLM("")
		fakeLLM.SetError(domain.NewTransientError("LLM quota exceeded", domain.ErrRateLimited))
		ragSvc := NewRAGService(mockRetrieval, fakeLLM, nil)

		_, err := ragSvc.Ask(ctx, ownerID, "Question", nil)
		assert.Error(t, err)
		assert.True(t, domain.IsTransient(err))
	})
}
