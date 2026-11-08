package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
)

// DocumentServiceConfig holds configuration options for document processing.
type DocumentServiceConfig struct {
	MaxUploadBytes int64
	JobMaxAttempts int
}

// DefaultDocumentService implements DocumentService.
type DefaultDocumentService struct {
	repo           repo.DocumentRepo
	maxUploadBytes int64
	jobMaxAttempts int
}

// NewDocumentService instantiates a DefaultDocumentService with configured limits.
func NewDocumentService(repo repo.DocumentRepo, cfg DocumentServiceConfig) *DefaultDocumentService {
	maxUpload := cfg.MaxUploadBytes
	if maxUpload <= 0 {
		maxUpload = 1048576 // 1 MB default
	}

	jobAttempts := cfg.JobMaxAttempts
	if jobAttempts <= 0 {
		jobAttempts = 5
	}

	return &DefaultDocumentService{
		repo:           repo,
		maxUploadBytes: maxUpload,
		jobMaxAttempts: jobAttempts,
	}
}

// Upload validates, hashes, and stores the document alongside an initial ingestion job.
func (s *DefaultDocumentService) Upload(ctx context.Context, ownerID string, filename string, content []byte) (*domain.Document, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}

	cleanFilename := filepath.Base(strings.TrimSpace(filename))
	if len(cleanFilename) == 0 || len(cleanFilename) > 255 {
		return nil, domain.NewValidationError("Filename must be between 1 and 255 characters.", nil)
	}

	ext := strings.ToLower(filepath.Ext(cleanFilename))
	if ext != ".txt" && ext != ".md" {
		return nil, domain.NewUnsupportedMediaTypeError("Unsupported file type. Only .txt and .md files are supported.")
	}

	size := int64(len(content))
	if size == 0 {
		return nil, domain.NewValidationError("Uploaded file is empty.", nil)
	}

	if size > s.maxUploadBytes {
		return nil, domain.NewPayloadTooLargeError(fmt.Sprintf("File size %d bytes exceeds maximum allowed limit of %d bytes.", size, s.maxUploadBytes))
	}

	if !utf8.Valid(content) {
		return nil, domain.NewValidationError("File content is not valid UTF-8.", nil)
	}

	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])

	doc := &domain.Document{
		OwnerID:     ownerID,
		Filename:    cleanFilename,
		SizeBytes:   size,
		ContentHash: contentHash,
		RawContent:  string(content),
		Status:      "queued",
		ChunkCount:  0,
	}

	_, err := s.repo.CreateWithJob(ctx, doc, s.jobMaxAttempts)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GetByID retrieves a document by ID scoped to the authenticated owner.
func (s *DefaultDocumentService) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}
	if id == uuid.Nil {
		return nil, domain.NewValidationError("Invalid document ID.", nil)
	}

	return s.repo.GetByID(ctx, ownerID, id)
}

// List returns a cursor-paginated list of documents owned by ownerID.
func (s *DefaultDocumentService) List(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Document, *string, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, nil, domain.NewUnauthorizedError("Authentication required.")
	}

	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	cursorCreatedAt, cursorID, err := DecodeCursor(cursor)
	if err != nil {
		return nil, nil, err
	}

	docs, err := s.repo.List(ctx, ownerID, limit, cursorCreatedAt, cursorID)
	if err != nil {
		return nil, nil, err
	}

	if len(docs) > limit {
		items := docs[:limit]
		lastItem := items[len(items)-1]
		nextCursor := EncodeCursor(lastItem.CreatedAt, lastItem.ID)
		return items, &nextCursor, nil
	}

	return docs, nil, nil
}

// Delete removes the document scoped to the owner, cascading to chunks and jobs.
func (s *DefaultDocumentService) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	if strings.TrimSpace(ownerID) == "" {
		return domain.NewUnauthorizedError("Authentication required.")
	}
	if id == uuid.Nil {
		return domain.NewValidationError("Invalid document ID.", nil)
	}

	return s.repo.Delete(ctx, ownerID, id)
}
