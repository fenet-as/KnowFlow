package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDocumentRepo struct {
	createFn  func(ctx context.Context, doc *domain.Document, maxAttempts int) (*domain.Job, error)
	getByIDFn func(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error)
	listFn    func(ctx context.Context, ownerID string, limit int, cursorCreatedAt *time.Time, cursorID *uuid.UUID) ([]*domain.Document, error)
	deleteFn  func(ctx context.Context, ownerID string, id uuid.UUID) error
}

func (m *mockDocumentRepo) CreateWithJob(ctx context.Context, doc *domain.Document, maxAttempts int) (*domain.Job, error) {
	if m.createFn != nil {
		return m.createFn(ctx, doc, maxAttempts)
	}
	doc.ID = uuid.New()
	doc.CreatedAt = time.Now().UTC()
	doc.UpdatedAt = time.Now().UTC()
	return &domain.Job{ID: uuid.New(), DocumentID: doc.ID, Status: "queued"}, nil
}

func (m *mockDocumentRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, ownerID, id)
	}
	return nil, domain.NewNotFoundError("Document not found.")
}

func (m *mockDocumentRepo) List(ctx context.Context, ownerID string, limit int, cursorCreatedAt *time.Time, cursorID *uuid.UUID) ([]*domain.Document, error) {
	if m.listFn != nil {
		return m.listFn(ctx, ownerID, limit, cursorCreatedAt, cursorID)
	}
	return nil, nil
}

func (m *mockDocumentRepo) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, ownerID, id)
	}
	return nil
}

func TestDocumentService_Upload(t *testing.T) {
	mockRepo := &mockDocumentRepo{}
	svc := NewDocumentService(mockRepo, DocumentServiceConfig{
		MaxUploadBytes: 1024,
		JobMaxAttempts: 5,
	})

	ctx := context.Background()
	ownerID := "user_clerk_123"

	t.Run("successful .txt upload", func(t *testing.T) {
		content := []byte("Hello KnowFlow world!")
		doc, err := svc.Upload(ctx, ownerID, "notes.txt", content)
		require.NoError(t, err)
		assert.Equal(t, "notes.txt", doc.Filename)
		assert.Equal(t, int64(len(content)), doc.SizeBytes)
		assert.Equal(t, "queued", doc.Status)
		assert.NotEmpty(t, doc.ContentHash)
	})

	t.Run("successful .md upload", func(t *testing.T) {
		content := []byte("# Architecture\nThis is markdown.")
		doc, err := svc.Upload(ctx, ownerID, "architecture.md", content)
		require.NoError(t, err)
		assert.Equal(t, "architecture.md", doc.Filename)
		assert.Equal(t, int64(len(content)), doc.SizeBytes)
	})

	t.Run("unsupported file extension (.pdf)", func(t *testing.T) {
		_, err := svc.Upload(ctx, ownerID, "document.pdf", []byte("pdf binary"))
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeUnsupportedMediaType, appErr.Code)
	})

	t.Run("empty content returns 400 validation error", func(t *testing.T) {
		_, err := svc.Upload(ctx, ownerID, "empty.txt", []byte{})
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeValidationFailed, appErr.Code)
	})

	t.Run("payload exceeding limit returns 413", func(t *testing.T) {
		hugeContent := []byte(strings.Repeat("a", 2048))
		_, err := svc.Upload(ctx, ownerID, "huge.txt", hugeContent)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodePayloadTooLarge, appErr.Code)
	})

	t.Run("invalid UTF-8 returns 400", func(t *testing.T) {
		invalidUTF8 := []byte{0xff, 0xfe, 0xfd}
		_, err := svc.Upload(ctx, ownerID, "bad.txt", invalidUTF8)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeValidationFailed, appErr.Code)
	})

	t.Run("unauthenticated owner_id returns 401", func(t *testing.T) {
		_, err := svc.Upload(ctx, "", "notes.txt", []byte("content"))
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeUnauthorized, appErr.Code)
	})
}

func TestDocumentService_GetByID(t *testing.T) {
	docID := uuid.New()
	mockRepo := &mockDocumentRepo{
		getByIDFn: func(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
			if ownerID == "user_1" && id == docID {
				return &domain.Document{ID: docID, OwnerID: "user_1", Filename: "test.md"}, nil
			}
			return nil, domain.NewNotFoundError("Document not found.")
		},
	}
	svc := NewDocumentService(mockRepo, DocumentServiceConfig{})

	t.Run("found for owner", func(t *testing.T) {
		doc, err := svc.GetByID(context.Background(), "user_1", docID)
		require.NoError(t, err)
		assert.Equal(t, docID, doc.ID)
	})

	t.Run("cross tenant returns 404", func(t *testing.T) {
		_, err := svc.GetByID(context.Background(), "user_2", docID)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeNotFound, appErr.Code)
	})
}

func TestDocumentService_List(t *testing.T) {
	now := time.Now().UTC()
	doc1 := &domain.Document{ID: uuid.New(), CreatedAt: now.Add(-1 * time.Minute)}
	doc2 := &domain.Document{ID: uuid.New(), CreatedAt: now.Add(-2 * time.Minute)}
	doc3 := &domain.Document{ID: uuid.New(), CreatedAt: now.Add(-3 * time.Minute)}

	mockRepo := &mockDocumentRepo{
		listFn: func(ctx context.Context, ownerID string, limit int, cursorCreatedAt *time.Time, cursorID *uuid.UUID) ([]*domain.Document, error) {
			// Returns 3 items for limit=2 to simulate has_more
			return []*domain.Document{doc1, doc2, doc3}, nil
		},
	}
	svc := NewDocumentService(mockRepo, DocumentServiceConfig{})

	items, nextCursor, err := svc.List(context.Background(), "user_1", 2, "")
	require.NoError(t, err)
	assert.Len(t, items, 2)
	require.NotNil(t, nextCursor)
	assert.NotEmpty(t, *nextCursor)

	// Decode the cursor to verify it encodes the 2nd item (last in page)
	curTime, curID, err := DecodeCursor(*nextCursor)
	require.NoError(t, err)
	assert.Equal(t, doc2.ID, *curID)
	assert.True(t, doc2.CreatedAt.Equal(*curTime))
}

func TestDocumentService_Delete(t *testing.T) {
	docID := uuid.New()
	deleted := false
	mockRepo := &mockDocumentRepo{
		deleteFn: func(ctx context.Context, ownerID string, id uuid.UUID) error {
			if ownerID == "user_1" && id == docID {
				deleted = true
				return nil
			}
			return domain.NewNotFoundError("Document not found.")
		},
	}
	svc := NewDocumentService(mockRepo, DocumentServiceConfig{})

	t.Run("delete success", func(t *testing.T) {
		err := svc.Delete(context.Background(), "user_1", docID)
		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("delete cross-tenant returns 404", func(t *testing.T) {
		err := svc.Delete(context.Background(), "user_other", docID)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.CodeNotFound, appErr.Code)
	})
}
