package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/embed"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
)

// RetrievalConfig configures retrieval limits and the relevance gating threshold.
type RetrievalConfig struct {
	TopK     int
	MinScore float64
}

// DefaultRetrievalService implements RetrievalService.
type DefaultRetrievalService struct {
	embedder  embed.Embedder
	chunkRepo repo.ChunkRepo
	topK      int
	minScore  float64
	logger    *slog.Logger
}

// NewRetrievalService constructs a DefaultRetrievalService.
func NewRetrievalService(
	embedder embed.Embedder,
	chunkRepo repo.ChunkRepo,
	cfg RetrievalConfig,
	logger *slog.Logger,
) *DefaultRetrievalService {
	topK := cfg.TopK
	if topK <= 0 {
		topK = 6
	}

	minScore := cfg.MinScore
	if minScore <= 0 {
		minScore = 0.65
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultRetrievalService{
		embedder:  embedder,
		chunkRepo: chunkRepo,
		topK:      topK,
		minScore:  minScore,
		logger:    logger,
	}
}

// Retrieve embeds the query with taskType RETRIEVAL_QUERY, runs similarity search,
// applies the relevance gate, and returns the gated retrieval result.
func (s *DefaultRetrievalService) Retrieve(
	ctx context.Context,
	ownerID string,
	query string,
	documentIDs []uuid.UUID,
) (*RetrievalResult, error) {
	cleanOwner := strings.TrimSpace(ownerID)
	if cleanOwner == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}

	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return nil, domain.NewValidationError("Query cannot be empty.", nil)
	}

	// 1. Embed query with RETRIEVAL_QUERY task type
	queryVectors, err := s.embedder.Embed(ctx, []string{cleanQuery}, embed.EmbedKindQuery)
	if err != nil {
		return nil, err
	}
	if len(queryVectors) == 0 {
		return nil, domain.NewInternalError("Failed to generate embedding for query.", nil)
	}
	queryVec := queryVectors[0]

	// 2. Similarity search scoped to owner and ready documents
	matches, err := s.chunkRepo.SimilaritySearch(ctx, cleanOwner, queryVec, documentIDs, s.topK)
	if err != nil {
		return nil, err
	}

	// 3. Relevance Gate
	if len(matches) == 0 {
		s.logger.Info("Retrieval relevance gate: zero chunks found",
			"owner_id", cleanOwner,
			"query", cleanQuery,
			"has_relevant_context", false,
		)
		return &RetrievalResult{
			HasRelevantContext: false,
			Matches:            []domain.ChunkMatch{},
			TopScore:           0.0,
			Query:              cleanQuery,
		}, nil
	}

	topScore := matches[0].Score
	scores := make([]float64, len(matches))
	for i, m := range matches {
		scores[i] = m.Score
	}

	hasRelevantContext := topScore >= s.minScore

	s.logger.Info("Retrieval similarity scores",
		"owner_id", cleanOwner,
		"query", cleanQuery,
		"match_count", len(matches),
		"top_score", topScore,
		"threshold", s.minScore,
		"scores", scores,
		"has_relevant_context", hasRelevantContext,
	)

	if !hasRelevantContext {
		return &RetrievalResult{
			HasRelevantContext: false,
			Matches:            []domain.ChunkMatch{},
			TopScore:           topScore,
			Query:              cleanQuery,
		}, nil
	}

	return &RetrievalResult{
		HasRelevantContext: true,
		Matches:            matches,
		TopScore:           topScore,
		Query:              cleanQuery,
	}, nil
}
