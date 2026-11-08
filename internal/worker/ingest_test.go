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
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	dbURL := getTestDatabaseURL()
	poolCfg := platform.DefaultDBPoolConfig()
	pool, err := platform.NewDBPool(ctx, dbURL, poolCfg)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dbURL, err)
		return
	}
	defer pool.Close()

	// Clean database tables before running ingestion tests
	_, _ = pool.Exec(ctx, "DELETE FROM documents")

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

		_ = docRepo.Delete(ctx, ownerID, doc.ID)
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

		_ = docRepo.Delete(ctx, ownerID, doc.ID)
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

	t.Run("transient error schedules retry with backoff through run_after", func(t *testing.T) {
		ownerID := "owner_transient_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "transient.md",
			SizeBytes:   64,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Content\n\nText to embed with rate limit.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5)
		require.NoError(t, err)

		// Claim once so attempts = 1
		claimedJob, err := jobRepo.Claim(ctx, "worker-1")
		require.NoError(t, err)
		require.NotNil(t, claimedJob)
		assert.Equal(t, job.ID, claimedJob.ID)

		transientEmbedder := &errorEmbedder{
			err: errors.New("HTTP 429 Too Many Requests: quota exceeded"),
		}
		transientHandler := worker.NewIngestionHandler(pool, chunker, transientEmbedder, jobRepo, nil).
			WithRetryPolicy(worker.RetryPolicy{
				BaseDelay:   200 * time.Millisecond,
				MaxDelay:    2 * time.Second,
				MaxAttempts: 5,
			})

		err = transientHandler.HandleJob(ctx, claimedJob)
		assert.Error(t, err)

		// Verify job is back in queued status with run_after in the future
		retriedJob, err := jobRepo.GetByID(ctx, claimedJob.ID)
		require.NoError(t, err)
		assert.Equal(t, "queued", retriedJob.Status)
		assert.Nil(t, retriedJob.LockedAt)
		assert.Nil(t, retriedJob.LockedBy)
		assert.True(t, retriedJob.RunAfter.After(time.Now().Add(-1*time.Second)))
		require.NotNil(t, retriedJob.LastError)
		assert.Contains(t, *retriedJob.LastError, "429")

		_ = docRepo.Delete(ctx, ownerID, doc.ID)
	})

	t.Run("transient error after attempt exhaustion marks document and job failed", func(t *testing.T) {
		ownerID := "owner_exhaust_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "exhaust.md",
			SizeBytes:   64,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Content\n\nWill exhaust attempts.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 1) // max_attempts = 1
		require.NoError(t, err)

		claimedJob, err := jobRepo.Claim(ctx, "worker-1")
		require.NoError(t, err)
		require.NotNil(t, claimedJob)
		assert.Equal(t, job.ID, claimedJob.ID)
		assert.Equal(t, 1, claimedJob.Attempts)

		transientEmbedder := &errorEmbedder{
			err: errors.New("HTTP 503 Service Unavailable"),
		}
		exhaustHandler := worker.NewIngestionHandler(pool, chunker, transientEmbedder, jobRepo, nil)

		err = exhaustHandler.HandleJob(ctx, claimedJob)
		assert.Error(t, err)

		// Since attempts (1) == max_attempts (1), should fail permanently
		failedJob, err := jobRepo.GetByID(ctx, claimedJob.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedJob.Status)
		require.NotNil(t, failedJob.LastError)
		assert.Contains(t, *failedJob.LastError, "503")

		failedDoc, err := docRepo.GetByID(ctx, ownerID, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedDoc.Status)
		require.NotNil(t, failedDoc.ErrorMessage)
		assert.Contains(t, *failedDoc.ErrorMessage, "503")

		_ = docRepo.Delete(ctx, ownerID, doc.ID)
	})

	t.Run("permanent error immediately marks document and job failed", func(t *testing.T) {
		ownerID := "owner_permanent_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "permanent.md",
			SizeBytes:   64,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Content\n\nDimension mismatch.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5) // max_attempts = 5
		require.NoError(t, err)

		claimedJob, err := jobRepo.Claim(ctx, "worker-1")
		require.NoError(t, err)
		require.NotNil(t, claimedJob)
		assert.Equal(t, job.ID, claimedJob.ID)
		assert.Equal(t, 1, claimedJob.Attempts)

		permanentEmbedder := &errorEmbedder{
			err: domain.NewPermanentError("dimension mismatch: got 512, expected 768", nil),
		}
		permHandler := worker.NewIngestionHandler(pool, chunker, permanentEmbedder, jobRepo, nil)

		err = permHandler.HandleJob(ctx, claimedJob)
		assert.Error(t, err)

		// Permanent error fails immediately without retrying
		failedJob, err := jobRepo.GetByID(ctx, claimedJob.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedJob.Status)

		failedDoc, err := docRepo.GetByID(ctx, ownerID, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, "failed", failedDoc.Status)
		require.NotNil(t, failedDoc.ErrorMessage)
		assert.Contains(t, *failedDoc.ErrorMessage, "dimension mismatch")

		_ = docRepo.Delete(ctx, ownerID, doc.ID)
	})

	t.Run("stale job recovery sweeper re-queues and processes crashed job (KF-26)", func(t *testing.T) {
		ownerID := "owner_stale_e2e_" + uuid.New().String()[:8]
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "stale_recovery.md",
			SizeBytes:   128,
			ContentHash: "hash_" + uuid.New().String(),
			RawContent:  "# Section 1\n\nRecovered after worker crash.",
		}

		job, err := docRepo.CreateWithJob(ctx, doc, 5)
		require.NoError(t, err)

		// Simulate worker crash: claim the job, then abandon it with locked_at in the past
		crashedWorkerID := "crashed-worker-pid-9999"
		claimed, err := jobRepo.Claim(ctx, crashedWorkerID)
		require.NoError(t, err)
		require.NotNil(t, claimed)
		assert.Equal(t, job.ID, claimed.ID)
		assert.Equal(t, "running", claimed.Status)

		// Set locked_at to 1 hour ago
		_, err = pool.Exec(ctx, "UPDATE jobs SET locked_at = now() - interval '1 hour' WHERE id = $1", claimed.ID)
		require.NoError(t, err)

		// Start a worker pool with fast stale sweeper and fast poll interval
		poolCfg := worker.PoolConfig{
			WorkerCount:        1,
			PollInterval:       30 * time.Millisecond,
			JobTimeout:         2 * time.Second,
			ShutdownTimeout:    2 * time.Second,
			StaleCheckInterval: 40 * time.Millisecond,
			JobStaleAfter:      100 * time.Millisecond,
		}

		wp := worker.NewPool(poolCfg, jobRepo)
		wp.RegisterHandler("ingest_document", ingestHandler)

		err = wp.Start(context.Background())
		require.NoError(t, err)
		defer wp.Stop()

		// The sweeper should detect the job as stale, requeue it, and the worker in wp will claim and succeed
		assert.Eventually(t, func() bool {
			j, err := jobRepo.GetByID(ctx, claimed.ID)
			if err != nil {
				return false
			}
			return j.Status == "succeeded"
		}, 3*time.Second, 50*time.Millisecond, "stale job must be swept and completed by active worker")

		wp.Stop()

		// Document should also be 'ready'
		recoveredDoc, err := docRepo.GetByID(ctx, ownerID, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, "ready", recoveredDoc.Status)
		assert.Greater(t, recoveredDoc.ChunkCount, 0)

		_ = docRepo.Delete(ctx, ownerID, doc.ID)
	})
}
