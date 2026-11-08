package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// PostgresChunkRepo implements ChunkRepo using pgxpool and pgvector.
type PostgresChunkRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresChunkRepo constructs a PostgresChunkRepo.
func NewPostgresChunkRepo(pool *pgxpool.Pool) *PostgresChunkRepo {
	return &PostgresChunkRepo{pool: pool}
}

// BulkInsert inserts multiple chunks using a batched query pipeline.
func (r *PostgresChunkRepo) BulkInsert(ctx context.Context, chunks []*domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	insertSQL := `
		INSERT INTO chunks (
			document_id, owner_id, chunk_index, content, heading, token_count, embedding
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at;
	`

	for _, ch := range chunks {
		if ch.OwnerID == "" {
			return domain.NewValidationError("chunk owner_id is required", nil)
		}
		if ch.DocumentID == uuid.Nil {
			return domain.NewValidationError("chunk document_id is required", nil)
		}
		vec := pgvector.NewVector(ch.Embedding)
		batch.Queue(insertSQL, ch.DocumentID, ch.OwnerID, ch.ChunkIndex, ch.Content, ch.Heading, ch.TokenCount, vec)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for _, ch := range chunks {
		err := br.QueryRow().Scan(&ch.ID, &ch.CreatedAt)
		if err != nil {
			return domain.NewInternalError("failed to insert chunk in batch", err)
		}
	}

	return br.Close()
}

// DeleteByDocument removes all chunks belonging to a document and owner.
func (r *PostgresChunkRepo) DeleteByDocument(ctx context.Context, ownerID string, documentID uuid.UUID) error {
	query := `DELETE FROM chunks WHERE document_id = $1 AND owner_id = $2;`
	_, err := r.pool.Exec(ctx, query, documentID, ownerID)
	if err != nil {
		return domain.NewInternalError("failed to delete chunks by document", err)
	}
	return nil
}

// GetByID returns the details of a single chunk scoped to its owner.
func (r *PostgresChunkRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.ChunkDetail, error) {
	query := `
		SELECT c.id, c.document_id, d.filename, c.chunk_index, c.heading, c.content
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.id = $1 AND c.owner_id = $2;
	`

	detail := &domain.ChunkDetail{}
	err := r.pool.QueryRow(ctx, query, id, ownerID).Scan(
		&detail.ID,
		&detail.DocumentID,
		&detail.DocumentName,
		&detail.ChunkIndex,
		&detail.Heading,
		&detail.Content,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("Chunk not found.")
		}
		return nil, domain.NewInternalError("failed to fetch chunk detail", err)
	}

	return detail, nil
}

// SimilaritySearch retrieves the top-k chunks nearest to the query embedding.
// Filters by owner, document 'ready' status, and optional document IDs.
// Score is computed as 1 - cosine_distance.
func (r *PostgresChunkRepo) SimilaritySearch(
	ctx context.Context,
	ownerID string,
	embedding []float32,
	documentIDs []uuid.UUID,
	limit int,
) ([]domain.ChunkMatch, error) {
	if ownerID == "" {
		return nil, domain.NewUnauthorizedError("Authentication required.")
	}
	if limit <= 0 {
		limit = 6
	}

	query := `
		SELECT c.id, c.document_id, d.filename, c.heading, c.content,
		       1 - (c.embedding <=> $1) AS score
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.owner_id = $2
		  AND d.status = 'ready'
		  AND ($3::uuid[] IS NULL OR c.document_id = ANY($3))
		ORDER BY c.embedding <=> $1
		LIMIT $4;
	`

	var docIDsParam any
	if len(documentIDs) > 0 {
		docIDsParam = documentIDs
	} else {
		docIDsParam = nil
	}

	queryVector := pgvector.NewVector(embedding)

	rows, err := r.pool.Query(ctx, query, queryVector, ownerID, docIDsParam, limit)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("similarity search query failed: %v", err), err)
	}
	defer rows.Close()

	var matches []domain.ChunkMatch
	for rows.Next() {
		var match domain.ChunkMatch
		err := rows.Scan(
			&match.ID,
			&match.DocumentID,
			&match.DocumentName,
			&match.Heading,
			&match.Content,
			&match.Score,
		)
		if err != nil {
			return nil, domain.NewInternalError("failed to scan chunk match row", err)
		}
		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		return nil, domain.NewInternalError("error iterating similarity search rows", err)
	}

	return matches, nil
}
