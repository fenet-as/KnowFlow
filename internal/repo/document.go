package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDocumentRepo implements DocumentRepo using pgxpool.
type PostgresDocumentRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresDocumentRepo creates a new PostgresDocumentRepo.
func NewPostgresDocumentRepo(pool *pgxpool.Pool) *PostgresDocumentRepo {
	return &PostgresDocumentRepo{pool: pool}
}

// CreateWithJob inserts the document (status 'queued') and its initial ingestion job in a single transaction.
func (r *PostgresDocumentRepo) CreateWithJob(ctx context.Context, doc *domain.Document, maxAttempts int) (*domain.Job, error) {
	if doc == nil {
		return nil, domain.NewValidationError("document must not be nil", nil)
	}
	if doc.OwnerID == "" {
		return nil, domain.NewValidationError("owner_id must not be empty", nil)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, domain.NewInternalError("failed to begin transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	insertDocSQL := `
		INSERT INTO documents (
			owner_id, filename, size_bytes, content_hash, raw_content, status, chunk_count
		) VALUES ($1, $2, $3, $4, $5, 'queued', 0)
		RETURNING id, status, chunk_count, created_at, updated_at;
	`

	err = tx.QueryRow(ctx, insertDocSQL,
		doc.OwnerID,
		doc.Filename,
		doc.SizeBytes,
		doc.ContentHash,
		doc.RawContent,
	).Scan(
		&doc.ID,
		&doc.Status,
		&doc.ChunkCount,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation
			return nil, domain.NewValidationError(fmt.Sprintf("document constraint violation: %s", pgErr.Message), err)
		}
		return nil, domain.NewInternalError("failed to insert document", err)
	}

	insertJobSQL := `
		INSERT INTO jobs (
			document_id, type, status, attempts, max_attempts, run_after
		) VALUES ($1, 'ingest_document', 'queued', 0, $2, now())
		RETURNING id, document_id, type, status, attempts, max_attempts, run_after, created_at;
	`

	job := &domain.Job{}
	err = tx.QueryRow(ctx, insertJobSQL, doc.ID, maxAttempts).Scan(
		&job.ID,
		&job.DocumentID,
		&job.Type,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.RunAfter,
		&job.CreatedAt,
	)
	if err != nil {
		return nil, domain.NewInternalError("failed to enqueue ingestion job", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, domain.NewInternalError("failed to commit document upload transaction", err)
	}

	return job, nil
}

// GetByID fetches a single document owned by ownerID. Returns domain.ErrNotFound if not found or cross-tenant.
func (r *PostgresDocumentRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
	query := `
		SELECT id, owner_id, filename, size_bytes, content_hash, raw_content,
		       status, error_message, chunk_count, created_at, updated_at
		FROM documents
		WHERE id = $1 AND owner_id = $2;
	`

	doc := &domain.Document{}
	err := r.pool.QueryRow(ctx, query, id, ownerID).Scan(
		&doc.ID,
		&doc.OwnerID,
		&doc.Filename,
		&doc.SizeBytes,
		&doc.ContentHash,
		&doc.RawContent,
		&doc.Status,
		&doc.ErrorMessage,
		&doc.ChunkCount,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("Document not found.")
		}
		return nil, domain.NewInternalError("failed to query document", err)
	}

	return doc, nil
}

// List returns a cursor-paginated page of documents for ownerID, newest first.
// Fetches up to limit + 1 rows so the caller can determine if more pages exist.
func (r *PostgresDocumentRepo) List(
	ctx context.Context,
	ownerID string,
	limit int,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
) ([]*domain.Document, error) {
	query := `
		SELECT id, owner_id, filename, size_bytes, content_hash, raw_content,
		       status, error_message, chunk_count, created_at, updated_at
		FROM documents
		WHERE owner_id = $1
		  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3))
		ORDER BY created_at DESC, id DESC
		LIMIT $4;
	`

	fetchLimit := limit + 1
	rows, err := r.pool.Query(ctx, query, ownerID, cursorCreatedAt, cursorID, fetchLimit)
	if err != nil {
		return nil, domain.NewInternalError("failed to list documents", err)
	}
	defer rows.Close()

	var docs []*domain.Document
	for rows.Next() {
		doc := &domain.Document{}
		err := rows.Scan(
			&doc.ID,
			&doc.OwnerID,
			&doc.Filename,
			&doc.SizeBytes,
			&doc.ContentHash,
			&doc.RawContent,
			&doc.Status,
			&doc.ErrorMessage,
			&doc.ChunkCount,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)
		if err != nil {
			return nil, domain.NewInternalError("failed to scan document row", err)
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, domain.NewInternalError("error iterating document rows", err)
	}

	return docs, nil
}

// Delete removes a document owned by ownerID. Cascading foreign keys remove associated chunks and jobs.
// Returns domain.ErrNotFound if no document matched id and ownerID.
func (r *PostgresDocumentRepo) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	query := `DELETE FROM documents WHERE id = $1 AND owner_id = $2;`

	tag, err := r.pool.Exec(ctx, query, id, ownerID)
	if err != nil {
		return domain.NewInternalError("failed to delete document", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("Document not found.")
	}

	return nil
}
