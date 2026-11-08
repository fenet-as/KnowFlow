package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeUnitVector(dim int, primaryIndex int) []float32 {
	vec := make([]float32, dim)
	if primaryIndex >= 0 && primaryIndex < dim {
		vec[primaryIndex] = 1.0
	}
	return vec
}

func TestPostgresChunkRepo_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dbURL := getTestDatabaseURL()
	poolCfg := platform.DefaultDBPoolConfig()
	pool, err := platform.NewDBPool(ctx, dbURL, poolCfg)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dbURL, err)
		return
	}
	defer pool.Close()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	chunkRepo := repo.NewPostgresChunkRepo(pool)

	dim := 768
	ownerA := "owner_chunk_A_" + uuid.New().String()[:8]
	ownerB := "owner_chunk_B_" + uuid.New().String()[:8]

	// 1. Create a ready document for Owner A
	docA1 := &domain.Document{
		OwnerID:     ownerA,
		Filename:    "docA1.txt",
		SizeBytes:   100,
		ContentHash: "hashA1",
		RawContent:  "Content for doc A1",
	}
	_, err = docRepo.CreateWithJob(ctx, docA1, 5)
	require.NoError(t, err)

	// Mark docA1 as 'ready'
	_, err = pool.Exec(ctx, "UPDATE documents SET status = 'ready' WHERE id = $1", docA1.ID)
	require.NoError(t, err)

	// Create a second ready document for Owner A
	docA2 := &domain.Document{
		OwnerID:     ownerA,
		Filename:    "docA2.txt",
		SizeBytes:   100,
		ContentHash: "hashA2",
		RawContent:  "Content for doc A2",
	}
	_, err = docRepo.CreateWithJob(ctx, docA2, 5)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE documents SET status = 'ready' WHERE id = $1", docA2.ID)
	require.NoError(t, err)

	// Create a document for Owner A that is still 'processing' (not ready)
	docPending := &domain.Document{
		OwnerID:     ownerA,
		Filename:    "docPending.txt",
		SizeBytes:   100,
		ContentHash: "hashPending",
		RawContent:  "Pending content",
	}
	_, err = docRepo.CreateWithJob(ctx, docPending, 5)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE documents SET status = 'processing' WHERE id = $1", docPending.ID)
	require.NoError(t, err)

	// Create a ready document for Owner B
	docB := &domain.Document{
		OwnerID:     ownerB,
		Filename:    "docB.txt",
		SizeBytes:   100,
		ContentHash: "hashB",
		RawContent:  "Content for doc B",
	}
	_, err = docRepo.CreateWithJob(ctx, docB, 5)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE documents SET status = 'ready' WHERE id = $1", docB.ID)
	require.NoError(t, err)

	// 2. Prepare chunks with known orthogonal unit vectors
	// Vector 0: perfectly aligned with query vector [1, 0, 0, ...] -> cosine similarity 1.0
	// Vector 1: orthogonal [0, 1, 0, ...] -> cosine similarity 0.0
	// Vector 2: orthogonal [0, 0, 1, ...]
	heading := "Section Heading"
	chunkA1_0 := &domain.Chunk{
		DocumentID: docA1.ID,
		OwnerID:    ownerA,
		ChunkIndex: 0,
		Content:    "High relevance chunk for doc A1",
		Heading:    &heading,
		TokenCount: 10,
		Embedding:  makeUnitVector(dim, 0), // [1, 0, 0, ...]
	}
	chunkA1_1 := &domain.Chunk{
		DocumentID: docA1.ID,
		OwnerID:    ownerA,
		ChunkIndex: 1,
		Content:    "Lower relevance chunk for doc A1",
		Heading:    &heading,
		TokenCount: 10,
		Embedding:  makeUnitVector(dim, 1), // [0, 1, 0, ...]
	}
	chunkA2_0 := &domain.Chunk{
		DocumentID: docA2.ID,
		OwnerID:    ownerA,
		ChunkIndex: 0,
		Content:    "Chunk from doc A2",
		TokenCount: 10,
		Embedding:  makeUnitVector(dim, 2), // [0, 0, 1, ...]
	}
	chunkPending := &domain.Chunk{
		DocumentID: docPending.ID,
		OwnerID:    ownerA,
		ChunkIndex: 0,
		Content:    "Chunk from pending doc",
		TokenCount: 10,
		Embedding:  makeUnitVector(dim, 0), // perfectly matches query, but doc status is 'processing'
	}
	chunkB := &domain.Chunk{
		DocumentID: docB.ID,
		OwnerID:    ownerB,
		ChunkIndex: 0,
		Content:    "Chunk from owner B",
		TokenCount: 10,
		Embedding:  makeUnitVector(dim, 0), // perfectly matches query, but belongs to owner B
	}

	err = chunkRepo.BulkInsert(ctx, []*domain.Chunk{chunkA1_0, chunkA1_1, chunkA2_0, chunkPending, chunkB})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, chunkA1_0.ID)

	// 3. Test GetByID
	detail, err := chunkRepo.GetByID(ctx, ownerA, chunkA1_0.ID)
	require.NoError(t, err)
	assert.Equal(t, chunkA1_0.ID, detail.ID)
	assert.Equal(t, "docA1.txt", detail.DocumentName)
	assert.Equal(t, "Section Heading", *detail.Heading)

	// Cross-tenant GetByID returns 404
	_, err = chunkRepo.GetByID(ctx, ownerB, chunkA1_0.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// 4. Test SimilaritySearch
	queryVec := makeUnitVector(dim, 0) // Looking for vectors near [1, 0, 0, ...]

	// Search without document filter
	matches, err := chunkRepo.SimilaritySearch(ctx, ownerA, queryVec, nil, 6)
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	// Proves ordering: chunkA1_0 (score ~1.0) must come first
	assert.Equal(t, chunkA1_0.ID, matches[0].ID)
	assert.InDelta(t, 1.0, matches[0].Score, 0.001)

	// Proves owner isolation: chunkB is NOT in results
	for _, m := range matches {
		assert.NotEqual(t, chunkB.ID, m.ID, "chunk belonging to owner B must not appear in owner A's search")
	}

	// Proves document status 'ready' filter: chunkPending is NOT in results
	for _, m := range matches {
		assert.NotEqual(t, chunkPending.ID, m.ID, "chunk from non-ready document must not appear in search")
	}

	// 5. Test SimilaritySearch with specific documentIDs filter
	filteredMatches, err := chunkRepo.SimilaritySearch(ctx, ownerA, queryVec, []uuid.UUID{docA2.ID}, 6)
	require.NoError(t, err)
	require.Len(t, filteredMatches, 1)
	assert.Equal(t, chunkA2_0.ID, filteredMatches[0].ID)

	// 6. Test DeleteByDocument
	err = chunkRepo.DeleteByDocument(ctx, ownerA, docA1.ID)
	require.NoError(t, err)

	_, err = chunkRepo.GetByID(ctx, ownerA, chunkA1_0.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
