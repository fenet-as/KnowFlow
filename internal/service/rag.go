package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/llm"
	"github.com/google/uuid"
)

// DefaultRAGService coordinates retrieval, prompt building, LLM generation, and citation extraction.
type DefaultRAGService struct {
	retrievalSvc RetrievalService
	llmClient    llm.LLM
	logger       *slog.Logger
}

// NewRAGService creates an initialized DefaultRAGService.
func NewRAGService(retrievalSvc RetrievalService, llmClient llm.LLM, logger *slog.Logger) *DefaultRAGService {
	if logger == nil {
		logger = slog.Default()
	}
	return &DefaultRAGService{
		retrievalSvc: retrievalSvc,
		llmClient:    llmClient,
		logger:       logger,
	}
}

// Ask executes the stateless RAG pipeline without conversation history.
func (s *DefaultRAGService) Ask(
	ctx context.Context,
	ownerID string,
	question string,
	documentIDs []uuid.UUID,
) (*domain.Answer, error) {
	return s.AskWithHistory(ctx, ownerID, question, documentIDs, nil)
}

// AskWithHistory executes the RAG pipeline taking prior turns into account:
// 1. Retrieves top-k chunks with relevance gating using only the latest question.
// 2. If no relevant context is found, immediately returns fixed response without calling the LLM.
// 3. Otherwise, constructs the prompt including conversation history, queries the LLM, extracts citations, and validates grounding.
func (s *DefaultRAGService) AskWithHistory(
	ctx context.Context,
	ownerID string,
	question string,
	documentIDs []uuid.UUID,
	priorTurns []llm.ConversationTurn,
) (*domain.Answer, error) {
	cleanOwner := strings.TrimSpace(ownerID)
	if cleanOwner == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}

	cleanQuestion := strings.TrimSpace(question)
	if cleanQuestion == "" {
		return nil, domain.NewValidationError("Question cannot be empty.", nil)
	}

	// 1. Retrieve candidates through relevance gate (using latest question only)
	retrievalRes, err := s.retrievalSvc.Retrieve(ctx, cleanOwner, cleanQuestion, documentIDs)
	if err != nil {
		return nil, err
	}

	// 2. No-context path: return fixed message with NO LLM call
	if !retrievalRes.HasRelevantContext || len(retrievalRes.Matches) == 0 {
		return &domain.Answer{
			Answer:    FixedNoRelevantContextMessage,
			Grounded:  false,
			Citations: []domain.Citation{},
		}, nil
	}

	// 3. Build grounded prompt including dialogue history
	prompt := llm.BuildPrompt(cleanQuestion, retrievalRes.Matches, priorTurns)

	// 4. Generate answer via LLM
	rawAnswer, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// 5. Extract citations and verify grounding
	cleanedAnswer, citations, grounded := llm.ExtractCitations(rawAnswer, retrievalRes.Matches)
	if !grounded {
		s.logger.Info("LLM generated answer lacked valid citations, returning fixed fallback",
			"owner_id", cleanOwner,
			"question", cleanQuestion,
			"raw_answer", rawAnswer,
		)
	}

	// Ensure citations slice is never nil in JSON response
	if citations == nil {
		citations = []domain.Citation{}
	}

	return &domain.Answer{
		Answer:    cleanedAnswer,
		Grounded:  grounded,
		Citations: citations,
	}, nil
}
