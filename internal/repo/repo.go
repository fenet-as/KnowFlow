package repo

import (
	"context"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
)

// Owner-scoping convention rules (KF-11):
// 1. Every repository method MUST accept ownerID string as a top-level parameter.
// 2. Every SELECT, UPDATE, DELETE query MUST include `WHERE owner_id = $x` in its predicate.
// 3. For tables without direct owner_id (e.g. messages), query MUST join parent (conversations) and filter by parent owner_id.
// 4. Accessing another user's resource MUST return domain.ErrNotFound / HTTP 404 (never 403 Forbidden).

// DocumentRepo defines the multi-tenant repository contract for documents.
type DocumentRepo interface {
	CreateWithJob(ctx context.Context, doc *domain.Document, maxAttempts int) (*domain.Job, error)
	GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error)
	List(ctx context.Context, ownerID string, limit int, cursorCreatedAt *time.Time, cursorID *uuid.UUID) ([]*domain.Document, error)
	Delete(ctx context.Context, ownerID string, id uuid.UUID) error
}

// ChunkRepo defines the multi-tenant repository contract for chunk storage and vector retrieval.
type ChunkRepo interface {
	BulkInsert(ctx context.Context, chunks []domain.Chunk) error
	DeleteByDocument(ctx context.Context, ownerID string, documentID uuid.UUID) error
	GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Chunk, error)
}

// JobRepo defines the repository contract for the transactional async queue.
type JobRepo interface {
	Enqueue(ctx context.Context, documentID uuid.UUID) error
	Complete(ctx context.Context, jobID uuid.UUID) error
	Fail(ctx context.Context, jobID uuid.UUID, lastError string) error
	RetryLater(ctx context.Context, jobID uuid.UUID, runAfter time.Time, lastError string) error
}

// ConversationRepo defines the multi-tenant repository contract for chat threads and turns.
type ConversationRepo interface {
	Create(ctx context.Context, conv *domain.Conversation) error
	GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Conversation, error)
	List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Conversation, string, error)
	Delete(ctx context.Context, ownerID string, id uuid.UUID) error
}
