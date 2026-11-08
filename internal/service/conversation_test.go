package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConvRepo struct {
	convs    map[uuid.UUID]*domain.Conversation
	messages map[uuid.UUID][]*domain.Message
}

func newMockConvRepo() *mockConvRepo {
	return &mockConvRepo{
		convs:    make(map[uuid.UUID]*domain.Conversation),
		messages: make(map[uuid.UUID][]*domain.Message),
	}
}

func (m *mockConvRepo) Create(ctx context.Context, conv *domain.Conversation) error {
	conv.ID = uuid.New()
	conv.CreatedAt = time.Now().UTC()
	conv.UpdatedAt = conv.CreatedAt
	m.convs[conv.ID] = conv
	return nil
}

func (m *mockConvRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Conversation, error) {
	c, ok := m.convs[id]
	if !ok || c.OwnerID != ownerID {
		return nil, domain.NewNotFoundError("Conversation not found.")
	}
	return c, nil
}

func (m *mockConvRepo) List(ctx context.Context, ownerID string, limit int, cursorUpdatedAt *time.Time, cursorID *uuid.UUID) ([]*domain.Conversation, error) {
	var list []*domain.Conversation
	for _, c := range m.convs {
		if c.OwnerID == ownerID {
			list = append(list, c)
		}
	}
	return list, nil
}

func (m *mockConvRepo) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	c, ok := m.convs[id]
	if !ok || c.OwnerID != ownerID {
		return domain.NewNotFoundError("Conversation not found.")
	}
	delete(m.convs, id)
	delete(m.messages, id)
	return nil
}

func (m *mockConvRepo) CreateMessage(ctx context.Context, ownerID string, msg *domain.Message) error {
	c, ok := m.convs[msg.ConversationID]
	if !ok || c.OwnerID != ownerID {
		return domain.NewNotFoundError("Conversation not found.")
	}
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now().UTC()
	m.messages[msg.ConversationID] = append(m.messages[msg.ConversationID], msg)
	return nil
}

func (m *mockConvRepo) ListMessages(ctx context.Context, ownerID string, convID uuid.UUID, limit int, cursorCreatedAt *time.Time, cursorID *uuid.UUID) ([]*domain.Message, error) {
	c, ok := m.convs[convID]
	if !ok || c.OwnerID != ownerID {
		return nil, domain.NewNotFoundError("Conversation not found.")
	}
	return m.messages[convID], nil
}

func (m *mockConvRepo) GetWithMessages(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error) {
	c, err := m.GetByID(ctx, ownerID, id)
	if err != nil {
		return nil, nil, err
	}
	return c, m.messages[id], nil
}

func TestConversationService_Create(t *testing.T) {
	repo := newMockConvRepo()
	svc := service.NewConversationService(repo)

	// Unauthenticated
	_, err := svc.Create(context.Background(), "", "Test")
	assert.Error(t, err)

	// Default title
	c1, err := svc.Create(context.Background(), "user1", "")
	require.NoError(t, err)
	assert.Equal(t, "New conversation", c1.Title)

	// Custom title
	c2, err := svc.Create(context.Background(), "user1", "Specific discussion")
	require.NoError(t, err)
	assert.Equal(t, "Specific discussion", c2.Title)
}

func TestConversationService_GetAndMessages(t *testing.T) {
	repo := newMockConvRepo()
	svc := service.NewConversationService(repo)
	ctx := context.Background()

	conv, err := svc.Create(ctx, "user1", "My Chat")
	require.NoError(t, err)

	// Add user message
	msg1, err := svc.CreateMessage(ctx, "user1", conv.ID, "user", "Hello there", nil)
	require.NoError(t, err)
	assert.Equal(t, "user", msg1.Role)
	assert.Equal(t, "Hello there", msg1.Content)

	// Add assistant message with citations
	msg2, err := svc.CreateMessage(ctx, "user1", conv.ID, "assistant", "Answer here [1]", []map[string]any{{"marker": 1}})
	require.NoError(t, err)
	assert.Equal(t, "assistant", msg2.Role)

	// Invalid role
	_, err = svc.CreateMessage(ctx, "user1", conv.ID, "invalid_role", "Bad role", nil)
	assert.Error(t, err)

	// Empty content
	_, err = svc.CreateMessage(ctx, "user1", conv.ID, "user", "   ", nil)
	assert.Error(t, err)

	// Get with messages
	c, msgs, err := svc.GetByID(ctx, "user1", conv.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, c.ID)
	assert.Len(t, msgs, 2)

	// Cross-tenant get
	_, _, err = svc.GetByID(ctx, "user2", conv.ID, 10)
	assert.Error(t, err)

	// Delete
	err = svc.Delete(ctx, "user1", conv.ID)
	require.NoError(t, err)

	// Get after delete
	_, _, err = svc.GetByID(ctx, "user1", conv.ID, 10)
	assert.Error(t, err)
}
