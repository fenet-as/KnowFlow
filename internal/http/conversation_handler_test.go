package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConversationService struct {
	createFn       func(ctx context.Context, ownerID string, title string) (*domain.Conversation, error)
	getByIDFn      func(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error)
	listFn         func(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Conversation, *string, error)
	deleteFn       func(ctx context.Context, ownerID string, id uuid.UUID) error
	createMsgFn    func(ctx context.Context, ownerID string, convID uuid.UUID, role string, content string, citations any) (*domain.Message, error)
	listMessagesFn func(ctx context.Context, ownerID string, convID uuid.UUID, limit int, cursor string) ([]*domain.Message, *string, error)
}

func (m *mockConversationService) Create(ctx context.Context, ownerID string, title string) (*domain.Conversation, error) {
	if m.createFn != nil {
		return m.createFn(ctx, ownerID, title)
	}
	return nil, nil
}

func (m *mockConversationService) GetByID(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, ownerID, id, messageLimit)
	}
	return nil, nil, domain.NewNotFoundError("Conversation not found.")
}

func (m *mockConversationService) List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Conversation, *string, error) {
	if m.listFn != nil {
		return m.listFn(ctx, ownerID, limit, cursor)
	}
	return nil, nil, nil
}

func (m *mockConversationService) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, ownerID, id)
	}
	return nil
}

func (m *mockConversationService) CreateMessage(ctx context.Context, ownerID string, convID uuid.UUID, role string, content string, citations any) (*domain.Message, error) {
	if m.createMsgFn != nil {
		return m.createMsgFn(ctx, ownerID, convID, role, content, citations)
	}
	return nil, nil
}

func (m *mockConversationService) ListMessages(ctx context.Context, ownerID string, convID uuid.UUID, limit int, cursor string) ([]*domain.Message, *string, error) {
	if m.listMessagesFn != nil {
		return m.listMessagesFn(ctx, ownerID, convID, limit, cursor)
	}
	return nil, nil, nil
}

func setupTestConversationRouter(svc *mockConversationService, ownerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewConversationHandler(svc)

	authMiddleware := func(c *gin.Context) {
		if ownerID != "" {
			c.Request = c.Request.WithContext(auth.WithOwnerID(c.Request.Context(), ownerID))
		}
		c.Next()
	}

	api := r.Group("/api/v1")
	api.Use(authMiddleware)
	{
		api.POST("/conversations", handler.Create)
		api.GET("/conversations", handler.List)
		api.GET("/conversations/:id", handler.GetByID)
		api.DELETE("/conversations/:id", handler.Delete)
		api.GET("/conversations/:id/messages", handler.ListMessages)
	}

	return r
}

func TestConversationHandler_Create(t *testing.T) {
	t.Run("success with title", func(t *testing.T) {
		convID := uuid.New()
		now := time.Now().UTC()
		svc := &mockConversationService{
			createFn: func(ctx context.Context, ownerID, title string) (*domain.Conversation, error) {
				assert.Equal(t, "user_123", ownerID)
				assert.Equal(t, "Q4 Planning", title)
				return &domain.Conversation{
					ID:        convID,
					OwnerID:   ownerID,
					Title:     title,
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		body := `{"title": "Q4 Planning"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp ConversationResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, convID, resp.ID)
		assert.Equal(t, "Q4 Planning", resp.Title)
	})

	t.Run("unauthorized", func(t *testing.T) {
		svc := &mockConversationService{}
		r := setupTestConversationRouter(svc, "") // no auth

		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", bytes.NewBufferString(`{}`))
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestConversationHandler_List(t *testing.T) {
	t.Run("success returns items and cursor", func(t *testing.T) {
		conv1 := &domain.Conversation{
			ID:        uuid.New(),
			OwnerID:   "user_123",
			Title:     "Thread 1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		nextCursor := "cursor_token"

		svc := &mockConversationService{
			listFn: func(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Conversation, *string, error) {
				assert.Equal(t, "user_123", ownerID)
				assert.Equal(t, 10, limit)
				return []*domain.Conversation{conv1}, &nextCursor, nil
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?limit=10", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp ConversationListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, conv1.ID, resp.Items[0].ID)
		require.NotNil(t, resp.NextCursor)
		assert.Equal(t, "cursor_token", *resp.NextCursor)
	})

	t.Run("invalid limit returns 400", func(t *testing.T) {
		svc := &mockConversationService{}
		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?limit=-5", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestConversationHandler_GetByID(t *testing.T) {
	t.Run("success with messages", func(t *testing.T) {
		convID := uuid.New()
		msgID := uuid.New()
		now := time.Now().UTC()

		svc := &mockConversationService{
			getByIDFn: func(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error) {
				assert.Equal(t, "user_123", ownerID)
				assert.Equal(t, convID, id)
				return &domain.Conversation{
						ID:        convID,
						OwnerID:   ownerID,
						Title:     "Thread",
						CreatedAt: now,
						UpdatedAt: now,
					}, []*domain.Message{
						{
							ID:             msgID,
							ConversationID: convID,
							Role:           "user",
							Content:        "Hello RAG",
							Citations:      []any{},
							CreatedAt:      now,
						},
					}, nil
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+convID.String(), nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp ConversationDetailResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, convID, resp.ID)
		assert.Len(t, resp.Messages, 1)
		assert.Equal(t, "Hello RAG", resp.Messages[0].Content)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		svc := &mockConversationService{
			getByIDFn: func(ctx context.Context, ownerID string, id uuid.UUID, messageLimit int) (*domain.Conversation, []*domain.Message, error) {
				return nil, nil, domain.NewNotFoundError("Conversation not found.")
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid uuid returns 400", func(t *testing.T) {
		svc := &mockConversationService{}
		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/not-a-uuid", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestConversationHandler_Delete(t *testing.T) {
	t.Run("success returns 204", func(t *testing.T) {
		convID := uuid.New()
		svc := &mockConversationService{
			deleteFn: func(ctx context.Context, ownerID string, id uuid.UUID) error {
				assert.Equal(t, "user_123", ownerID)
				assert.Equal(t, convID, id)
				return nil
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+convID.String(), nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		svc := &mockConversationService{
			deleteFn: func(ctx context.Context, ownerID string, id uuid.UUID) error {
				return domain.NewNotFoundError("Conversation not found.")
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+uuid.New().String(), nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestConversationHandler_ListMessages(t *testing.T) {
	t.Run("success returns messages", func(t *testing.T) {
		convID := uuid.New()
		msgID := uuid.New()
		now := time.Now().UTC()

		svc := &mockConversationService{
			listMessagesFn: func(ctx context.Context, ownerID string, cID uuid.UUID, limit int, cursor string) ([]*domain.Message, *string, error) {
				assert.Equal(t, "user_123", ownerID)
				assert.Equal(t, convID, cID)
				return []*domain.Message{
					{
						ID:             msgID,
						ConversationID: convID,
						Role:           "assistant",
						Content:        "Answer here",
						Citations:      []any{},
						CreatedAt:      now,
					},
				}, nil, nil
			},
		}

		r := setupTestConversationRouter(svc, "user_123")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+convID.String()+"/messages", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp MessageListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, msgID, resp.Items[0].ID)
		assert.Equal(t, "assistant", resp.Items[0].Role)
	})
}
