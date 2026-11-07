package repo

import (
	"context"

	"github.com/fenet-as/KnowFlow/internal/domain"
)

// DocumentRepository defines the repository interface for document persistence.
// Every method requires ownerID for tenant isolation.
type DocumentRepository interface {
	Create(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, ownerID string, id string) (*domain.Document, error)
}

