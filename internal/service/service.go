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
