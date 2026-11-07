package service

import (
	"context"

	"github.com/fenet-as/KnowFlow/internal/domain"
)

// DocumentService defines the interface for document operations.
// Services never import Gin and always take context.Context first.
type DocumentService interface {
	Upload(ctx context.Context, ownerID string, filename string, content []byte) (*domain.Document, error)
}

