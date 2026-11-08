package service

import (
	"context"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
)

// DocumentService defines the business interface for document operations.
// Services never import Gin and always take context.Context first.
type DocumentService interface {
	Upload(ctx context.Context, ownerID string, filename string, content []byte) (*domain.Document, error)
	GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error)
	List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Document, *string, error)
	Delete(ctx context.Context, ownerID string, id uuid.UUID) error
}

// RetrievalResult holds the output of query retrieval after the relevance gate is evaluated.
type RetrievalResult struct {
	HasRelevantContext bool                `json:"has_relevant_context"`
	Matches            []domain.ChunkMatch `json:"matches"`
	TopScore           float64             `json:"top_score"`
	Query              string              `json:"query"`
}

// FixedNoRelevantContextMessage is the standardized response text when nothing relevant is found.
const FixedNoRelevantContextMessage = "I could not find this in your documents."

// RetrievalService defines the contract for query embedding, vector similarity search, and relevance gating.
type RetrievalService interface {
	Retrieve(ctx context.Context, ownerID string, query string, documentIDs []uuid.UUID) (*RetrievalResult, error)
}
