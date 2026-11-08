package service

import (
	"context"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/llm"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
)

// ConversationService defines the business operations for chat threads and messages (KF-28, KF-29).
type ConversationService interface {
	Create(ctx context.Context, ownerID string, title string) (*domain.Conversation, error)
	GetByID(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error)
	List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Conversation, *string, error)
	Delete(ctx context.Context, ownerID string, id uuid.UUID) error
	CreateMessage(ctx context.Context, ownerID string, convID uuid.UUID, role string, content string, citations any) (*domain.Message, error)
	ListMessages(ctx context.Context, ownerID string, convID uuid.UUID, limit int, cursor string) ([]*domain.Message, *string, error)
	Ask(ctx context.Context, ownerID string, convID uuid.UUID, content string, documentIDs []uuid.UUID) (*domain.Message, *domain.Message, error)
}

// DefaultConversationService implements ConversationService.
type DefaultConversationService struct {
	repo         repo.ConversationRepo
	ragSvc       RAGService
	historyTurns int
}

// NewConversationService constructs a new DefaultConversationService.
func NewConversationService(repo repo.ConversationRepo, ragSvc RAGService, historyTurns int) *DefaultConversationService {
	if historyTurns <= 0 {
		historyTurns = 6
	}
	return &DefaultConversationService{
		repo:         repo,
		ragSvc:       ragSvc,
		historyTurns: historyTurns,
	}
}

// Create creates a new chat thread for ownerID.
func (s *DefaultConversationService) Create(ctx context.Context, ownerID string, title string) (*domain.Conversation, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}

	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = "New conversation"
	}
	if len(cleanTitle) > 255 {
		cleanTitle = cleanTitle[:255]
	}

	conv := &domain.Conversation{
		OwnerID: ownerID,
		Title:   cleanTitle,
	}

	if err := s.repo.Create(ctx, conv); err != nil {
		return nil, err
	}
	return conv, nil
}

// GetByID retrieves a conversation along with its messages, scoped to the owner.
func (s *DefaultConversationService) GetByID(
	ctx context.Context,
	ownerID string,
	id uuid.UUID,
	messageLimit int,
) (*domain.Conversation, []*domain.Message, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, nil, domain.NewUnauthorizedError("Authentication required.")
	}
	if messageLimit <= 0 {
		messageLimit = 50
	} else if messageLimit > 100 {
		messageLimit = 100
	}

	return s.repo.GetWithMessages(ctx, ownerID, id, messageLimit)
}

// List returns a cursor-paginated list of conversations for ownerID.
func (s *DefaultConversationService) List(
	ctx context.Context,
	ownerID string,
	limit int,
	cursor string,
) ([]*domain.Conversation, *string, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, nil, domain.NewUnauthorizedError("Authentication required.")
	}

	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	cursorUpdatedAt, cursorID, err := DecodeCursor(cursor)
	if err != nil {
		return nil, nil, err
	}

	convs, err := s.repo.List(ctx, ownerID, limit, cursorUpdatedAt, cursorID)
	if err != nil {
		return nil, nil, err
	}

	if len(convs) > limit {
		items := convs[:limit]
		last := items[len(items)-1]
		next := EncodeCursor(last.UpdatedAt, last.ID)
		return items, &next, nil
	}

	return convs, nil, nil
}

// Delete removes a conversation and its messages.
func (s *DefaultConversationService) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	if strings.TrimSpace(ownerID) == "" {
		return domain.NewUnauthorizedError("Authentication required.")
	}
	return s.repo.Delete(ctx, ownerID, id)
}

// CreateMessage adds a new turn to a conversation.
func (s *DefaultConversationService) CreateMessage(
	ctx context.Context,
	ownerID string,
	convID uuid.UUID,
	role string,
	content string,
	citations any,
) (*domain.Message, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}

	role = strings.ToLower(strings.TrimSpace(role))
	if role != "user" && role != "assistant" {
		return nil, domain.NewValidationError("role must be 'user' or 'assistant'", nil)
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, domain.NewValidationError("message content must not be empty", nil)
	}

	if citations == nil {
		citations = []any{}
	}

	msg := &domain.Message{
		ConversationID: convID,
		Role:           role,
		Content:        content,
		Citations:      citations,
	}

	if err := s.repo.CreateMessage(ctx, ownerID, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// ListMessages returns cursor-paginated messages for a conversation ordered chronologically.
func (s *DefaultConversationService) ListMessages(
	ctx context.Context,
	ownerID string,
	convID uuid.UUID,
	limit int,
	cursor string,
) ([]*domain.Message, *string, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, nil, domain.NewUnauthorizedError("Authentication required.")
	}

	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	cursorCreatedAt, cursorID, err := DecodeCursor(cursor)
	if err != nil {
		return nil, nil, err
	}

	messages, err := s.repo.ListMessages(ctx, ownerID, convID, limit, cursorCreatedAt, cursorID)
	if err != nil {
		return nil, nil, err
	}

	if len(messages) > limit {
		items := messages[:limit]
		last := items[len(items)-1]
		next := EncodeCursor(last.CreatedAt, last.ID)
		return items, &next, nil
	}

	return messages, nil, nil
}

// Ask performs a conversational ask (KF-29):
// 1. Validates inputs and retrieves the target conversation.
// 2. Fetches recent messages up to historyTurns, formatted oldest-first.
// 3. Executes the RAG pipeline taking prior turns into account, retrieving on the latest question only.
// 4. If the conversation has a default title, derives a new title from the first question (up to 60 chars).
// 5. Stores both the user question and the assistant response in one atomic transaction upon success.
func (s *DefaultConversationService) Ask(
	ctx context.Context,
	ownerID string,
	convID uuid.UUID,
	content string,
	documentIDs []uuid.UUID,
) (*domain.Message, *domain.Message, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, nil, domain.NewUnauthorizedError("Authentication required.")
	}

	cleanContent := strings.TrimSpace(content)
	if cleanContent == "" {
		return nil, nil, domain.NewValidationError("message content must not be empty", nil)
	}

	// 1. Verify conversation exists and fetch its metadata
	conv, err := s.repo.GetByID(ctx, ownerID, convID)
	if err != nil {
		return nil, nil, err
	}

	// 2. Retrieve recent message history
	recentMsgs, err := s.repo.GetRecentMessages(ctx, ownerID, convID, s.historyTurns)
	if err != nil {
		return nil, nil, err
	}

	priorTurns := make([]llm.ConversationTurn, 0, len(recentMsgs))
	for _, m := range recentMsgs {
		priorTurns = append(priorTurns, llm.ConversationTurn{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// 3. Coordinate with RAG service (retrieval uses latest question only)
	if s.ragSvc == nil {
		return nil, nil, domain.NewInternalError("RAG service is not configured", nil)
	}

	answer, err := s.ragSvc.AskWithHistory(ctx, ownerID, cleanContent, documentIDs, priorTurns)
	if err != nil {
		return nil, nil, err
	}

	// 4. Derive conversation title if first question or untitled
	var updateTitleTo string
	if conv.Title == "New conversation" || strings.TrimSpace(conv.Title) == "" {
		t := cleanContent
		if len(t) > 60 {
			t = t[:60]
		}
		updateTitleTo = t
	}

	// 5. Transactionally store user and assistant messages
	return s.repo.SaveTurn(
		ctx,
		ownerID,
		convID,
		cleanContent,
		answer.Answer,
		answer.Citations,
		updateTitleTo,
	)
}
