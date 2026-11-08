package service

import (
	"context"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
)

// ConversationService defines the business operations for chat threads and messages (KF-28).
type ConversationService interface {
	Create(ctx context.Context, ownerID string, title string) (*domain.Conversation, error)
	GetByID(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error)
	List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Conversation, *string, error)
	Delete(ctx context.Context, ownerID string, id uuid.UUID) error
	CreateMessage(ctx context.Context, ownerID string, convID uuid.UUID, role string, content string, citations any) (*domain.Message, error)
	ListMessages(ctx context.Context, ownerID string, convID uuid.UUID, limit int, cursor string) ([]*domain.Message, *string, error)
}

// DefaultConversationService implements ConversationService.
type DefaultConversationService struct {
	repo repo.ConversationRepo
}

// NewConversationService constructs a new DefaultConversationService.
func NewConversationService(repo repo.ConversationRepo) *DefaultConversationService {
	return &DefaultConversationService{repo: repo}
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
