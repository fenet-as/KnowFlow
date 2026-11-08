package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/llm"
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

func (m *mockConvRepo) GetRecentMessages(ctx context.Context, ownerID string, convID uuid.UUID, limit int) ([]*domain.Message, error) {
	c, ok := m.convs[convID]
	if !ok || c.OwnerID != ownerID {
		return nil, domain.NewNotFoundError("Conversation not found.")
	}
	msgs := m.messages[convID]
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

func (m *mockConvRepo) SaveTurn(ctx context.Context, ownerID string, convID uuid.UUID, userContent string, assistantContent string, citations any, updateTitleTo string) (*domain.Message, *domain.Message, error) {
	c, ok := m.convs[convID]
	if !ok || c.OwnerID != ownerID {
		return nil, nil, domain.NewNotFoundError("Conversation not found.")
	}
	if updateTitleTo != "" {
		c.Title = updateTitleTo
	}
	c.UpdatedAt = time.Now().UTC()

	userMsg := &domain.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        userContent,
		Citations:      []any{},
		CreatedAt:      time.Now().UTC(),
	}
	asstMsg := &domain.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "assistant",
		Content:        assistantContent,
		Citations:      citations,
		CreatedAt:      time.Now().UTC(),
	}
	m.messages[convID] = append(m.messages[convID], userMsg, asstMsg)
	return userMsg, asstMsg, nil
}

type mockRAGService struct {
	askFn func(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID, priorTurns []llm.ConversationTurn) (*domain.Answer, error)
}

func (m *mockRAGService) Ask(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID) (*domain.Answer, error) {
	return m.AskWithHistory(ctx, ownerID, question, documentIDs, nil)
}

func (m *mockRAGService) AskWithHistory(ctx context.Context, ownerID string, question string, documentIDs []uuid.UUID, priorTurns []llm.ConversationTurn) (*domain.Answer, error) {
	if m.askFn != nil {
		return m.askFn(ctx, ownerID, question, documentIDs, priorTurns)
	}
	return &domain.Answer{Answer: "Mock response", Grounded: true, Citations: []domain.Citation{}}, nil
}

func TestConversationService_Create(t *testing.T) {
	repo := newMockConvRepo()
	svc := service.NewConversationService(repo, nil, 6)

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
	svc := service.NewConversationService(repo, nil, 6)
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

func TestConversationService_Ask(t *testing.T) {
	ctx := context.Background()
	ownerID := "user_conv_ask"

	t.Run("first question sets conversation title and stores both turns", func(t *testing.T) {
		repo := newMockConvRepo()
		mockRAG := &mockRAGService{
			askFn: func(ctx context.Context, o string, q string, docIDs []uuid.UUID, turns []llm.ConversationTurn) (*domain.Answer, error) {
				assert.Equal(t, ownerID, o)
				assert.Equal(t, "What are the working hours?", q)
				assert.Empty(t, turns)
				return &domain.Answer{
					Answer:    "Working hours are 9am to 5pm [1].",
					Grounded:  true,
					Citations: []domain.Citation{{Marker: 1}},
				}, nil
			},
		}

		svc := service.NewConversationService(repo, mockRAG, 6)
		conv, err := svc.Create(ctx, ownerID, "")
		require.NoError(t, err)
		assert.Equal(t, "New conversation", conv.Title)

		uMsg, aMsg, err := svc.Ask(ctx, ownerID, conv.ID, "What are the working hours?", nil)
		require.NoError(t, err)
		assert.Equal(t, "user", uMsg.Role)
		assert.Equal(t, "What are the working hours?", uMsg.Content)
		assert.Equal(t, "assistant", aMsg.Role)
		assert.Equal(t, "Working hours are 9am to 5pm [1].", aMsg.Content)

		// Check updated title
		updatedConv, _, err := svc.GetByID(ctx, ownerID, conv.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, "What are the working hours?", updatedConv.Title)
	})

	t.Run("subsequent question includes prior turns in prompt", func(t *testing.T) {
		repo := newMockConvRepo()
		callCount := 0
		mockRAG := &mockRAGService{
			askFn: func(ctx context.Context, o string, q string, docIDs []uuid.UUID, turns []llm.ConversationTurn) (*domain.Answer, error) {
				callCount++
				if callCount == 1 {
					return &domain.Answer{
						Answer:   "First answer [1].",
						Grounded: true,
					}, nil
				}
				// Second call should have prior turns
				assert.Len(t, turns, 2)
				assert.Equal(t, "First question", turns[0].Content)
				assert.Equal(t, "First answer [1].", turns[1].Content)
				return &domain.Answer{
					Answer:   "Second answer [1].",
					Grounded: true,
				}, nil
			},
		}

		svc := service.NewConversationService(repo, mockRAG, 6)
		conv, err := svc.Create(ctx, ownerID, "Existing Title")
		require.NoError(t, err)

		_, _, err = svc.Ask(ctx, ownerID, conv.ID, "First question", nil)
		require.NoError(t, err)

		_, _, err = svc.Ask(ctx, ownerID, conv.ID, "Second question", nil)
		require.NoError(t, err)

		// Title should NOT have been overwritten since it had a custom title
		updatedConv, _, err := svc.GetByID(ctx, ownerID, conv.ID, 10)
		require.NoError(t, err)
		assert.Equal(t, "Existing Title", updatedConv.Title)
	})

	t.Run("RAG failure leaves database untouched", func(t *testing.T) {
		repo := newMockConvRepo()
		mockRAG := &mockRAGService{
			askFn: func(ctx context.Context, o string, q string, docIDs []uuid.UUID, turns []llm.ConversationTurn) (*domain.Answer, error) {
				return nil, domain.NewTransientError("LLM unavailable", domain.ErrUpstreamUnavailable)
			},
		}

		svc := service.NewConversationService(repo, mockRAG, 6)
		conv, err := svc.Create(ctx, ownerID, "New conversation")
		require.NoError(t, err)

		_, _, err = svc.Ask(ctx, ownerID, conv.ID, "Failing question", nil)
		assert.Error(t, err)

		// Verify no messages were stored
		_, msgs, err := svc.GetByID(ctx, ownerID, conv.ID, 10)
		require.NoError(t, err)
		assert.Empty(t, msgs)
	})
}
