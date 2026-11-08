package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDatabaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable"
	}
	return url
}

func TestPostgresDocumentRepo_Lifecycle(t *testing.T) {
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

	ownerA := "owner_test_A_" + uuid.New().String()[:8]
	ownerB := "owner_test_B_" + uuid.New().String()[:8]

	// 1. CreateWithJob
	doc := &domain.Document{
		OwnerID:     ownerA,
		Filename:    "handbook.md",
		SizeBytes:   128,
		ContentHash: "fakehash128",
		RawContent:  "# Employee Handbook\nWelcome to KnowFlow.",
	}

	job, err := docRepo.CreateWithJob(ctx, doc, 5)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.NotEqual(t, uuid.Nil, doc.ID)
	assert.Equal(t, "queued", doc.Status)
	assert.Equal(t, 0, doc.ChunkCount)
	assert.Equal(t, doc.ID, job.DocumentID)
	assert.Equal(t, "queued", job.Status)
	assert.Equal(t, 5, job.MaxAttempts)

	// Verify job exists in jobs table
	var jobStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", job.ID).Scan(&jobStatus)
	require.NoError(t, err)
	assert.Equal(t, "queued", jobStatus)

	// 2. GetByID (same owner)
	foundDoc, err := docRepo.GetByID(ctx, ownerA, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, foundDoc.ID)
	assert.Equal(t, "handbook.md", foundDoc.Filename)

	// 3. GetByID (cross-tenant isolation: ownerB querying ownerA's document returns 404)
	crossDoc, err := docRepo.GetByID(ctx, ownerB, doc.ID)
	assert.Error(t, err)
	assert.Nil(t, crossDoc)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// 4. Keyset Pagination
	// Insert two more documents for ownerA
	time.Sleep(10 * time.Millisecond)
	doc2 := &domain.Document{
		OwnerID:     ownerA,
		Filename:    "doc2.txt",
		SizeBytes:   50,
		ContentHash: "hash2",
		RawContent:  "Second document",
	}
	_, err = docRepo.CreateWithJob(ctx, doc2, 5)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	doc3 := &domain.Document{
		OwnerID:     ownerA,
		Filename:    "doc3.txt",
		SizeBytes:   60,
		ContentHash: "hash3",
		RawContent:  "Third document",
	}
	_, err = docRepo.CreateWithJob(ctx, doc3, 5)
	require.NoError(t, err)

	// Query page 1 with limit 2 (newest first: doc3, doc2)
	page1, err := docRepo.List(ctx, ownerA, 2, nil, nil)
	require.NoError(t, err)
	// List fetches limit + 1 (3 items) so service knows has_more
	assert.Len(t, page1, 3)
	assert.Equal(t, doc3.ID, page1[0].ID)
	assert.Equal(t, doc2.ID, page1[1].ID)

	// Query page 2 using cursor of doc2 (created_at, id)
	page2, err := docRepo.List(ctx, ownerA, 2, &page1[1].CreatedAt, &page1[1].ID)
	require.NoError(t, err)
	assert.Len(t, page2, 1)
	assert.Equal(t, doc.ID, page2[0].ID)

	// 5. Delete document and verify cascading deletes
	err = docRepo.Delete(ctx, ownerA, doc.ID)
	require.NoError(t, err)

	// Verify document is gone
	_, err = docRepo.GetByID(ctx, ownerA, doc.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Verify job was cascaded by ON DELETE CASCADE
	var remainingJobCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE id = $1", job.ID).Scan(&remainingJobCount)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingJobCount)

	// Delete non-existent or cross-tenant returns 404
	err = docRepo.Delete(ctx, ownerB, doc2.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
