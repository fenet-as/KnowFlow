package worker_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/chunk"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/embed"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/fenet-as/KnowFlow/internal/worker"
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

type errorEmbedder struct {
	err error
}

func (e *errorEmbedder) Embed(ctx context.Context, texts []string, kind embed.EmbedKind) ([][]float32, error) {
	return nil, e.err
}

func (e *errorEmbedder) Dimension() int {
	return 768
}

func TestIngestionHandler_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	jobRepo := repo.NewPostgresJobRepo(pool)
	fakeEmbedder := embed.NewFakeEmbedder(768)
	chunker := chunk.NewMarkdownChunker(chunk.ChunkerOptions{
		TargetTokens: 100,
		MaxTokens:    150,
		OverlapPct:   10,
	})

	ingestHandler := worker.NewIngestionHandler(pool, chunker, fakeEmbedder, jobRepo, nil)

	t.Run("successful ingestion transitions document to ready and job to succeeded", func(t *testing.T) {
		ownerID := "owner_ingest_test_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "handbook.md",
			SizeBytes:   256,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Introduction\n\nWelcome to KnowFlow.\n\n# Vacation Policy\n\nEmployees receive 20 days off.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5)
		require.NoError(t, err)

		// Execute handler
		err = ingestHandler.HandleJob(ctx, job)
		require.NoError(t, err)

		// Verify document state
		updatedDoc, err := docRepo.GetByID(ctx, ownerID, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, "ready", updatedDoc.Status)
		assert.Nil(t, updatedDoc.ErrorMessage)
		assert.Greater(t, updatedDoc.ChunkCount, 0)

		// Verify chunks in DB
		var chunkCountInDB int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&chunkCountInDB)
		require.NoError(t, err)
		assert.Equal(t, updatedDoc.ChunkCount, chunkCountInDB)

		// Verify job state
		updatedJob, err := jobRepo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", updatedJob.Status)
		assert.Nil(t, updatedJob.LockedAt)
		assert.Nil(t, updatedJob.LockedBy)
	})

	t.Run("running the same job twice yields the same chunk set idempotently", func(t *testing.T) {
		ownerID := "owner_idempotent_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "idempotent.md",
			SizeBytes:   128,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Section 1\n\nRepeatable chunking test content.\n\n# Section 2\n\nMore repeatable content.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5)
		require.NoError(t, err)

		// Run 1
		err = ingestHandler.HandleJob(ctx, job)
		require.NoError(t, err)

		var chunksRun1 int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&chunksRun1)
		require.NoError(t, err)

		// Run 2 on the same document and job
		err = ingestHandler.HandleJob(ctx, job)
		require.NoError(t, err)

		var chunksRun2 int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&chunksRun2)
		require.NoError(t, err)

		assert.Equal(t, chunksRun1, chunksRun2, "second run must yield identical chunk count without duplicate key conflicts")
	})

	t.Run("document deleted mid-job ends cleanly", func(t *testing.T) {
		ownerID := "owner_deleted_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "to_delete.md",
			SizeBytes:   64,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Ephemeral\n\nThis will be deleted.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5)
		require.NoError(t, err)

		// Delete document before handler finishes
		err = docRepo.Delete(ctx, ownerID, doc.ID)
		require.NoError(t, err)

		// Handler should detect missing document and return nil cleanly
		err = ingestHandler.HandleJob(ctx, job)
		assert.NoError(t, err, "deleted document must end cleanly without returning an error")
	})

	t.Run("embedding failure records readable error message", func(t *testing.T) {
		ownerID := "owner_fail_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "failing.md",
			SizeBytes:   64,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Content\n\nText to embed.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5)
		require.NoError(t, err)

		failingEmbedder := &errorEmbedder{
			err: errors.New("Gemini API quota exceeded"),
		}
		failingHandler := worker.NewIngestionHandler(pool, chunker, failingEmbedder, jobRepo, nil)

		err = failingHandler.HandleJob(ctx, job)
		assert.Error(t, err)

		// Verify document error message
		failedDoc, err := docRepo.GetByID(ctx, ownerID, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedDoc.Status)
		require.NotNil(t, failedDoc.ErrorMessage)
		assert.Contains(t, *failedDoc.ErrorMessage, "Gemini API quota exceeded")

		// Verify job error message
		failedJob, err := jobRepo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedJob.Status)
		require.NotNil(t, failedJob.LastError)
		assert.Contains(t, *failedJob.LastError, "Gemini API quota exceeded")
	})
}
