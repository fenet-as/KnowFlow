package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fenet-as/KnowFlow/internal/chunk"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/embed"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// IngestionHandler implements background document ingestion (KF-24, KF-25).
// It chunks document text, embeds in batches, and in one atomic transaction:
// replaces chunks, marks document ready with chunk count, and marks the job succeeded.
// Transient errors are retried with exponential backoff and jitter up to configured max attempts.
type IngestionHandler struct {
	pool        *pgxpool.Pool
	chunker     chunk.Chunker
	embedder    embed.Embedder
	jobRepo     repo.JobRepo
	logger      *slog.Logger
	retryPolicy RetryPolicy
}

// NewIngestionHandler constructs a new IngestionHandler with default retry policy.
func NewIngestionHandler(
	pool *pgxpool.Pool,
	chunker chunk.Chunker,
	embedder embed.Embedder,
	jobRepo repo.JobRepo,
	logger *slog.Logger,
) *IngestionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &IngestionHandler{
		pool:        pool,
		chunker:     chunker,
		embedder:    embedder,
		jobRepo:     jobRepo,
		logger:      logger,
		retryPolicy: DefaultRetryPolicy(),
	}
}

// WithRetryPolicy overrides the default retry policy on the handler.
func (h *IngestionHandler) WithRetryPolicy(p RetryPolicy) *IngestionHandler {
	h.retryPolicy = p
	return h
}

// HandleJob executes the full ingestion pipeline for an ingest_document job.
func (h *IngestionHandler) HandleJob(ctx context.Context, job *domain.Job) error {
	docID := job.DocumentID

	// 1. Fetch document from database
	docQuery := `
		SELECT id, owner_id, filename, raw_content, status
		FROM documents
		WHERE id = $1;
	`

	var (
		docOwnerID  string
		docFilename string
		docRaw      string
		docStatus   string
	)

	err := h.pool.QueryRow(ctx, docQuery, docID).Scan(
		&docID,
		&docOwnerID,
		&docFilename,
		&docRaw,
		&docStatus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// A document deleted before or mid-job ends cleanly (KF-24)
			h.logger.Info("Document deleted before or mid-job, ending ingestion cleanly",
				"document_id", docID,
				"job_id", job.ID,
			)
			return nil
		}
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to read document: %w", err))
	}

	// 2. Mark document as 'processing'
	markProcessingSQL := `
		UPDATE documents
		SET status = 'processing', updated_at = now()
		WHERE id = $1;
	`
	tag, err := h.pool.Exec(ctx, markProcessingSQL, docID)
	if err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to set status processing: %w", err))
	}
	if tag.RowsAffected() == 0 {
		// Document was deleted concurrently
		h.logger.Info("Document removed concurrently during status update", "document_id", docID)
		return nil
	}

	// 3. Segment content into deterministic chunks
	chunks, err := h.chunker.Split(docRaw)
	if err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("chunking failed: %w", err))
	}

	// 4. Generate embeddings in batches
	var embeddings [][]float32
	if len(chunks) > 0 {
		texts := make([]string, len(chunks))
		for i, ch := range chunks {
			texts[i] = ch.Content
		}

		embeddings, err = h.embedder.Embed(ctx, texts, embed.EmbedKindDocument)
		if err != nil {
			return h.handleError(ctx, docID, job, fmt.Errorf("embedding generation failed: %w", err))
		}

		if len(embeddings) != len(chunks) {
			return h.handleError(ctx, docID, job,
				domain.NewPermanentError(
					fmt.Sprintf("embedder returned %d vectors for %d chunks", len(embeddings), len(chunks)),
					nil,
				),
			)
		}
	}

	// 5. Atomic transaction: replace chunks, set document ready, mark job succeeded (KF-24)
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to begin transaction: %w", err))
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 5a. Clear any existing chunks for this document (ensures running twice yields exact same chunk set)
	deleteChunksSQL := `DELETE FROM chunks WHERE document_id = $1;`
	if _, err := tx.Exec(ctx, deleteChunksSQL, docID); err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to clear existing chunks: %w", err))
	}

	// 5b. Insert new chunks
	if len(chunks) > 0 {
		insertChunkSQL := `
			INSERT INTO chunks (
				document_id, owner_id, chunk_index, content, heading, token_count, embedding
			) VALUES ($1, $2, $3, $4, $5, $6, $7);
		`
		for i, ch := range chunks {
			vec := pgvector.NewVector(embeddings[i])
			_, err := tx.Exec(ctx, insertChunkSQL,
				docID,
				docOwnerID,
				ch.Index,
				ch.Content,
				ch.Heading,
				ch.TokenCount,
				vec,
			)
			if err != nil {
				return h.handleError(ctx, docID, job, fmt.Errorf("failed to insert chunk %d: %w", ch.Index, err))
			}
		}
	}

	// 5c. Set document ready with chunk count and clear error message
	updateDocSQL := `
		UPDATE documents
		SET status = 'ready', chunk_count = $2, error_message = NULL, updated_at = now()
		WHERE id = $1;
	`
	docTag, err := tx.Exec(ctx, updateDocSQL, docID, len(chunks))
	if err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to update document to ready: %w", err))
	}
	if docTag.RowsAffected() == 0 {
		// Document deleted mid-transaction, end cleanly
		h.logger.Info("Document deleted mid-transaction, ending job cleanly", "document_id", docID)
		return nil
	}

	// 5d. Mark job succeeded and clear locks
	updateJobSQL := `
		UPDATE jobs
		SET status = 'succeeded', locked_at = NULL, locked_by = NULL
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, updateJobSQL, job.ID); err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to complete job: %w", err))
	}

	// 5e. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return h.handleError(ctx, docID, job, fmt.Errorf("failed to commit ingestion transaction: %w", err))
	}

	h.logger.Info("Document ingestion succeeded",
		"document_id", docID,
		"filename", docFilename,
		"chunk_count", len(chunks),
		"job_id", job.ID,
	)

	return nil
}

// handleError processes errors according to the retry policy (KF-25):
// - Transient errors with attempts < max_attempts are retried with exponential backoff & jitter via RetryLater.
// - Permanent errors or exhausted attempts mark the document 'failed' with a readable error message and fail the job.
func (h *IngestionHandler) handleError(ctx context.Context, docID uuid.UUID, job *domain.Job, err error) error {
	isTransient := ClassifyError(err)
	maxAttempts := job.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = h.retryPolicy.MaxAttempts
	}

	readableErrMsg := err.Error()

	// Transient error with remaining attempts -> RetryLater
	if isTransient && job.Attempts < maxAttempts {
		delay := h.retryPolicy.CalculateDelay(job.Attempts)
		h.logger.Warn("Transient ingestion error, scheduling retry with backoff",
			"document_id", docID,
			"job_id", job.ID,
			"attempt", job.Attempts,
			"max_attempts", maxAttempts,
			"delay", delay,
			"error", readableErrMsg,
		)

		retryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.jobRepo.RetryLater(retryCtx, job.ID, delay, readableErrMsg)
		return err
	}

	// Permanent failure or retry attempts exhausted:
	h.logger.Error("Ingestion failed permanently or max attempts exhausted",
		"document_id", docID,
		"job_id", job.ID,
		"attempt", job.Attempts,
		"max_attempts", maxAttempts,
		"is_transient", isTransient,
		"error", readableErrMsg,
	)

	failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateDocSQL := `
		UPDATE documents
		SET status = 'failed', error_message = $2, updated_at = now()
		WHERE id = $1;
	`
	_, _ = h.pool.Exec(failCtx, updateDocSQL, docID, readableErrMsg)
	_ = h.jobRepo.Fail(failCtx, job.ID, readableErrMsg)

	return err
}
