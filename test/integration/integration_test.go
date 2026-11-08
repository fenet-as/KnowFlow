package integration_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/chunk"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/embed"
	"github.com/fenet-as/KnowFlow/internal/llm"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/fenet-as/KnowFlow/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findMigrationsDir() string {
	candidates := []string{"../../migrations", "migrations", "../migrations"}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return "migrations"
}

// connectIntegrationDB attempts to connect to PostgreSQL.
// It checks DATABASE_URL, port 5433 (docker compose default), and port 5432.
func connectIntegrationDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	urls := []string{}
	if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		urls = append(urls, envURL)
	}
	urls = append(urls,
		"postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable",
		"postgres://knowflow:knowflow@localhost:5432/knowflow?sslmode=disable",
		"postgres://knowflow:knowflow@127.0.0.1:5433/knowflow?sslmode=disable",
		"postgres://knowflow:knowflow@127.0.0.1:5432/knowflow?sslmode=disable",
	)

	var pool *pgxpool.Pool
	var connectedURL string
	var lastErr error

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, u := range urls {
		p, err := platform.NewDBPool(ctx, u, platform.DefaultDBPoolConfig())
		if err == nil {
			pool = p
			connectedURL = u
			break
		}
		lastErr = err
	}

	if pool == nil {
		t.Skipf("PostgreSQL is not reachable (last error: %v). Skipping integration tests against live DB.", lastErr)
		return nil, ""
	}

	// Ensure migrations are applied
	_ = platform.MigrateUp(connectedURL, findMigrationsDir())

	return pool, connectedURL
}

// TestIntegration_MultiTenantIsolation tests KF-36 multi-tenant data isolation:
// User A cannot list, read, search, or delete User B's documents, chunks, jobs, or conversations.
func TestIntegration_MultiTenantIsolation(t *testing.T) {
	pool, _ := connectIntegrationDB(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	chunkRepo := repo.NewPostgresChunkRepo(pool)
	convRepo := repo.NewPostgresConversationRepo(pool)
	fakeEmbedder := embed.NewFakeEmbedder(768)

	userA := "tenant_A_" + uuid.New().String()[:8]
	userB := "tenant_B_" + uuid.New().String()[:8]

	// 1. Create documents for both tenants
	docA := &domain.Document{
		OwnerID:     userA,
		Filename:    "handbook_A.md",
		SizeBytes:   100,
		ContentHash: "hash_A_" + uuid.New().String(),
		RawContent:  "# Handbook A\nConfidential policies for Tenant A.",
	}
	jobA, err := docRepo.CreateWithJob(ctx, docA, 3)
	require.NoError(t, err)

	docB := &domain.Document{
		OwnerID:     userB,
		Filename:    "financials_B.md",
		SizeBytes:   200,
		ContentHash: "hash_B_" + uuid.New().String(),
		RawContent:  "# Financials B\nConfidential revenue numbers for Tenant B.",
	}
	jobB, err := docRepo.CreateWithJob(ctx, docB, 3)
	require.NoError(t, err)

	// Documents must be in 'ready' status for chunks to be eligible in similarity search
	_, err = pool.Exec(ctx, "UPDATE documents SET status = 'ready' WHERE id = ANY($1)", []uuid.UUID{docA.ID, docB.ID})
	require.NoError(t, err)

	// Clean up at the end
	defer func() {
		_ = docRepo.Delete(ctx, userA, docA.ID)
		_ = docRepo.Delete(ctx, userB, docB.ID)
	}()

	// 2. Add chunks with identical vector embeddings for both documents
	embeddings, err := fakeEmbedder.Embed(ctx, []string{"shared topic text"}, embed.EmbedKindDocument)
	require.NoError(t, err)
	embedding := embeddings[0]

	headingA := "Section A"
	chunkA := &domain.Chunk{
		ID:          uuid.New(),
		DocumentID:  docA.ID,
		OwnerID:     userA,
		ChunkIndex:  0,
		Content:     "Tenant A secret strategy text.",
		Heading:     &headingA,
		TokenCount:  10,
		Embedding:   embedding,
	}
	require.NoError(t, chunkRepo.BulkInsert(ctx, []*domain.Chunk{chunkA}))

	headingB := "Section B"
	chunkB := &domain.Chunk{
		ID:          uuid.New(),
		DocumentID:  docB.ID,
		OwnerID:     userB,
		ChunkIndex:  0,
		Content:     "Tenant B secret strategy text.",
		Heading:     &headingB,
		TokenCount:  10,
		Embedding:   embedding,
	}
	require.NoError(t, chunkRepo.BulkInsert(ctx, []*domain.Chunk{chunkB}))

	// 3. Create conversations for both tenants
	convA := &domain.Conversation{
		OwnerID: userA,
		Title:   "Tenant A Conversation",
	}
	require.NoError(t, convRepo.Create(ctx, convA))
	msgA := &domain.Message{
		ConversationID: convA.ID,
		Role:           "user",
		Content:        "Hello from A",
	}
	require.NoError(t, convRepo.CreateMessage(ctx, userA, msgA))

	convB := &domain.Conversation{
		OwnerID: userB,
		Title:   "Tenant B Conversation",
	}
	require.NoError(t, convRepo.Create(ctx, convB))
	msgB := &domain.Message{
		ConversationID: convB.ID,
		Role:           "user",
		Content:        "Hello from B",
	}
	require.NoError(t, convRepo.CreateMessage(ctx, userB, msgB))

	defer func() {
		_ = convRepo.Delete(ctx, userA, convA.ID)
		_ = convRepo.Delete(ctx, userB, convB.ID)
	}()

	t.Run("Document isolation: listing", func(t *testing.T) {
		docsA, err := docRepo.List(ctx, userA, 50, nil, nil)
		require.NoError(t, err)
		for _, d := range docsA {
			assert.Equal(t, userA, d.OwnerID, "Tenant A list must not contain documents of other tenants")
			assert.NotEqual(t, docB.ID, d.ID)
		}

		docsB, err := docRepo.List(ctx, userB, 50, nil, nil)
		require.NoError(t, err)
		for _, d := range docsB {
			assert.Equal(t, userB, d.OwnerID, "Tenant B list must not contain documents of other tenants")
			assert.NotEqual(t, docA.ID, d.ID)
		}
	})

	t.Run("Document isolation: reading cross-tenant returns not found", func(t *testing.T) {
		// User A tries to read User B's document
		_, err := docRepo.GetByID(ctx, userA, docB.ID)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.CodeNotFound, appErr.Code)

		// User B tries to read User A's document
		_, err = docRepo.GetByID(ctx, userB, docA.ID)
		assert.Error(t, err)
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.CodeNotFound, appErr.Code)
	})

	t.Run("Document isolation: cross-tenant delete fails and does not delete document", func(t *testing.T) {
		// User A tries to delete User B's document
		err := docRepo.Delete(ctx, userA, docB.ID)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.CodeNotFound, appErr.Code)

		// Verify User B's document still exists intact
		bDoc, err := docRepo.GetByID(ctx, userB, docB.ID)
		require.NoError(t, err)
		assert.Equal(t, docB.ID, bDoc.ID)
	})

	t.Run("Chunk isolation: similarity search only returns caller's chunks", func(t *testing.T) {
		// Both chunks have identical embeddings, but search must be strictly scoped to owner_id
		matchesA, err := chunkRepo.SimilaritySearch(ctx, userA, embedding, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, matchesA)
		for _, m := range matchesA {
			assert.Equal(t, docA.ID, m.DocumentID, "User A similarity search must only return User A chunks")
			assert.NotEqual(t, docB.ID, m.DocumentID)
		}

		matchesB, err := chunkRepo.SimilaritySearch(ctx, userB, embedding, nil, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, matchesB)
		for _, m := range matchesB {
			assert.Equal(t, docB.ID, m.DocumentID, "User B similarity search must only return User B chunks")
			assert.NotEqual(t, docA.ID, m.DocumentID)
		}
	})

	t.Run("Chunk isolation: get by ID cross-tenant returns not found", func(t *testing.T) {
		_, err := chunkRepo.GetByID(ctx, userA, chunkB.ID)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.CodeNotFound, appErr.Code)

		_, err = chunkRepo.GetByID(ctx, userB, chunkA.ID)
		assert.Error(t, err)
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.CodeNotFound, appErr.Code)
	})

	t.Run("Job isolation: job ownership matches document owner", func(t *testing.T) {
		assert.Equal(t, docA.ID, jobA.DocumentID)
		assert.Equal(t, docB.ID, jobB.DocumentID)
	})

	t.Run("Conversation isolation: listing", func(t *testing.T) {
		convsA, err := convRepo.List(ctx, userA, 50, nil, nil)
		require.NoError(t, err)
		for _, c := range convsA {
			assert.Equal(t, userA, c.OwnerID)
			assert.NotEqual(t, convB.ID, c.ID)
		}

		convsB, err := convRepo.List(ctx, userB, 50, nil, nil)
		require.NoError(t, err)
		for _, c := range convsB {
			assert.Equal(t, userB, c.OwnerID)
			assert.NotEqual(t, convA.ID, c.ID)
		}
	})

	t.Run("Conversation isolation: read, create message, and delete cross-tenant returns not found", func(t *testing.T) {
		// Read cross-tenant
		_, err := convRepo.GetByID(ctx, userA, convB.ID)
		assert.Error(t, err)

		// Create message cross-tenant
		err = convRepo.CreateMessage(ctx, userA, &domain.Message{
			ConversationID: convB.ID,
			Role:           "user",
			Content:        "Intruder message",
		})
		assert.Error(t, err)

		// List messages cross-tenant
		_, err = convRepo.ListMessages(ctx, userA, convB.ID, 10, nil, nil)
		assert.Error(t, err)

		// Delete cross-tenant
		err = convRepo.Delete(ctx, userA, convB.ID)
		assert.Error(t, err)

		// Verify User B's conversation still exists
		bConv, err := convRepo.GetByID(ctx, userB, convB.ID)
		require.NoError(t, err)
		assert.Equal(t, convB.ID, bConv.ID)
	})
}

// TestIntegration_ConcurrentJobClaiming tests KF-36 concurrent claiming:
// Multiple workers claiming queued jobs concurrently using FOR UPDATE SKIP LOCKED.
// Proves no job is claimed twice and no race conditions occur.
func TestIntegration_ConcurrentJobClaiming(t *testing.T) {
	pool, _ := connectIntegrationDB(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	jobRepo := repo.NewPostgresJobRepo(pool)

	ownerID := "concurrent_test_owner_" + uuid.New().String()[:8]

	const totalJobs = 20
	jobIDs := make([]uuid.UUID, totalJobs)
	docIDs := make([]uuid.UUID, totalJobs)

	// Clean up after test
	defer func() {
		for _, docID := range docIDs {
			_ = docRepo.Delete(ctx, ownerID, docID)
		}
	}()

	for i := 0; i < totalJobs; i++ {
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    fmt.Sprintf("job_test_%d.txt", i),
			SizeBytes:   50,
			ContentHash: fmt.Sprintf("hash_concurrent_%d_%s", i, uuid.New().String()),
			RawContent:  fmt.Sprintf("Content for job %d", i),
		}
		job, err := docRepo.CreateWithJob(ctx, doc, 3)
		require.NoError(t, err)
		jobIDs[i] = job.ID
		docIDs[i] = doc.ID
	}

	const numWorkers = 5
	var claimedMu sync.Mutex
	claimedJobs := make(map[uuid.UUID]string) // jobID -> workerID
	duplicateCount := 0

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		workerID := fmt.Sprintf("worker-concurrency-%d", w)
		go func(wID string) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				job, err := jobRepo.Claim(ctx, wID)
				if err != nil || job == nil {
					// No more queued jobs to claim
					return
				}

				claimedMu.Lock()
				if existingWorker, exists := claimedJobs[job.ID]; exists {
					duplicateCount++
					t.Errorf("Job %s was claimed twice! First by %s, now by %s", job.ID, existingWorker, wID)
				} else {
					claimedJobs[job.ID] = wID
				}
				claimedMu.Unlock()

				// Small sleep to allow interleaving among workers
				time.Sleep(5 * time.Millisecond)
			}
		}(workerID)
	}

	wg.Wait()

	// Assertions
	assert.Equal(t, 0, duplicateCount, "No job should ever be claimed more than once")

	// Verify all created jobs were claimed
	claimedMu.Lock()
	claimedCount := 0
	for _, jID := range jobIDs {
		if _, ok := claimedJobs[jID]; ok {
			claimedCount++
		}
	}
	claimedMu.Unlock()

	assert.Equal(t, totalJobs, claimedCount, "All created jobs should be claimed across workers")
}

// TestIntegration_IngestionIdempotency tests KF-36 ingestion idempotency:
// Running ingestion twice on the same document yields identical chunk count and ready state,
// without creating orphan or duplicated chunks.
func TestIntegration_IngestionIdempotency(t *testing.T) {
	pool, _ := connectIntegrationDB(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	jobRepo := repo.NewPostgresJobRepo(pool)
	fakeEmbedder := embed.NewFakeEmbedder(768)
	chunker := chunk.NewMarkdownChunker(chunk.ChunkerOptions{
		TargetTokens: 100,
		MaxTokens:    150,
		OverlapPct:   10,
	})

	ingestHandler := worker.NewIngestionHandler(pool, chunker, fakeEmbedder, jobRepo, nil)

	ownerID := "idempotency_owner_" + uuid.New().String()[:8]
	doc := &domain.Document{
		OwnerID:     ownerID,
		Filename:    "idempotent_guide.md",
		SizeBytes:   300,
		ContentHash: "hash_idempotency_" + uuid.New().String(),
		RawContent:  "# Introduction\n\nWelcome to KnowFlow.\n\n# Features\n\nFull-text and vector search with citations.\n\n# FAQ\n\nFrequently asked questions.",
	}

	job, err := docRepo.CreateWithJob(ctx, doc, 5)
	require.NoError(t, err)

	defer func() {
		_ = docRepo.Delete(ctx, ownerID, doc.ID)
	}()

	// First ingestion run
	err = ingestHandler.HandleJob(ctx, job)
	require.NoError(t, err)

	docAfterFirst, err := docRepo.GetByID(ctx, ownerID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", docAfterFirst.Status)
	assert.Greater(t, docAfterFirst.ChunkCount, 0)
	initialChunkCount := docAfterFirst.ChunkCount

	// Verify chunks count in DB
	var dbChunkCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&dbChunkCount)
	require.NoError(t, err)
	assert.Equal(t, initialChunkCount, dbChunkCount)

	// Second ingestion run on the exact same document
	err = ingestHandler.HandleJob(ctx, job)
	require.NoError(t, err)

	docAfterSecond, err := docRepo.GetByID(ctx, ownerID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", docAfterSecond.Status)
	assert.Equal(t, initialChunkCount, docAfterSecond.ChunkCount, "Chunk count must remain identical after re-ingestion")

	// Verify no duplicated chunks in DB
	err = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&dbChunkCount)
	require.NoError(t, err)
	assert.Equal(t, initialChunkCount, dbChunkCount, "Database must not contain duplicate or orphaned chunks")
}

// TestIntegration_DeleteCascade tests KF-36 delete cascade:
// Deleting a document cascades to chunks and jobs.
// Deleting a conversation cascades to messages.
func TestIntegration_DeleteCascade(t *testing.T) {
	pool, _ := connectIntegrationDB(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	chunkRepo := repo.NewPostgresChunkRepo(pool)
	convRepo := repo.NewPostgresConversationRepo(pool)
	fakeEmbedder := embed.NewFakeEmbedder(768)

	ownerID := "cascade_owner_" + uuid.New().String()[:8]

	t.Run("Document delete cascades to chunks and jobs", func(t *testing.T) {
		doc := &domain.Document{
			OwnerID:     ownerID,
			Filename:    "cascade_doc.md",
			SizeBytes:   80,
			ContentHash: "hash_cascade_" + uuid.New().String(),
			RawContent:  "# Cascade Test\nWill be deleted.",
		}
		job, err := docRepo.CreateWithJob(ctx, doc, 3)
		require.NoError(t, err)

		embeddings, err := fakeEmbedder.Embed(ctx, []string{"test chunk"}, embed.EmbedKindDocument)
		require.NoError(t, err)

		chunk1 := &domain.Chunk{
			ID:          uuid.New(),
			DocumentID:  doc.ID,
			OwnerID:     ownerID,
			ChunkIndex:  0,
			Content:     "Chunk to be cascade deleted",
			TokenCount:  5,
			Embedding:   embeddings[0],
		}
		require.NoError(t, chunkRepo.BulkInsert(ctx, []*domain.Chunk{chunk1}))

		// Verify rows exist prior to deletion
		var chunksCountBefore, jobsCountBefore int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&chunksCountBefore)
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE document_id = $1", doc.ID).Scan(&jobsCountBefore)
		assert.Equal(t, 1, chunksCountBefore)
		assert.Equal(t, 1, jobsCountBefore)

		// Delete document
		err = docRepo.Delete(ctx, ownerID, doc.ID)
		require.NoError(t, err)

		// Verify cascading deletion
		var chunksCountAfter, jobsCountAfter int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM chunks WHERE document_id = $1", doc.ID).Scan(&chunksCountAfter)
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE id = $1", job.ID).Scan(&jobsCountAfter)
		assert.Equal(t, 0, chunksCountAfter, "Chunks must be deleted via cascade")
		assert.Equal(t, 0, jobsCountAfter, "Jobs must be deleted via cascade")
	})

	t.Run("Conversation delete cascades to messages", func(t *testing.T) {
		conv := &domain.Conversation{
			OwnerID: ownerID,
			Title:   "Cascade Conversation",
		}
		require.NoError(t, convRepo.Create(ctx, conv))

		err := convRepo.CreateMessage(ctx, ownerID, &domain.Message{
			ConversationID: conv.ID,
			Role:           "user",
			Content:        "Message 1",
		})
		require.NoError(t, err)
		err = convRepo.CreateMessage(ctx, ownerID, &domain.Message{
			ConversationID: conv.ID,
			Role:           "assistant",
			Content:        "Message 2",
		})
		require.NoError(t, err)

		var msgsBefore int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE conversation_id = $1", conv.ID).Scan(&msgsBefore)
		assert.Equal(t, 2, msgsBefore)

		// Delete conversation
		err = convRepo.Delete(ctx, ownerID, conv.ID)
		require.NoError(t, err)

		var msgsAfter int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE conversation_id = $1", conv.ID).Scan(&msgsAfter)
		assert.Equal(t, 0, msgsAfter, "Messages must be deleted via cascade")
	})
}

// TestIntegration_EndToEndUploadToReadyPipeline tests KF-36 end-to-end pipeline:
// Upload to ready with FakeEmbedder, similarity search, and RAG answering with citations.
func TestIntegration_EndToEndUploadToReadyPipeline(t *testing.T) {
	pool, _ := connectIntegrationDB(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docRepo := repo.NewPostgresDocumentRepo(pool)
	jobRepo := repo.NewPostgresJobRepo(pool)
	chunkRepo := repo.NewPostgresChunkRepo(pool)

	fakeEmbedder := embed.NewFakeEmbedder(768)
	chunker := chunk.NewMarkdownChunker(chunk.ChunkerOptions{
		TargetTokens: 100,
		MaxTokens:    150,
		OverlapPct:   10,
	})

	docService := service.NewDocumentService(docRepo, service.DocumentServiceConfig{
		MaxUploadBytes: 10 * 1024 * 1024,
		JobMaxAttempts: 3,
	})
	ingestHandler := worker.NewIngestionHandler(pool, chunker, fakeEmbedder, jobRepo, nil)

	ownerID := "pipeline_user_" + uuid.New().String()[:8]
	markdownContent := `# Company Vacation Policy

Full-time employees receive 20 days of paid time off per calendar year.
PTO begins accruing on the first day of employment.

# Remote Work Guidelines

Employees may work remotely up to two days per week with manager approval.
Core collaboration hours are 10:00 AM to 3:00 PM.`

	// 1. Upload document through service
	doc, err := docService.Upload(ctx, ownerID, "handbook.md", []byte(markdownContent))
	require.NoError(t, err)
	assert.Equal(t, "queued", doc.Status)
	assert.Equal(t, 0, doc.ChunkCount)

	defer func() {
		_ = docRepo.Delete(ctx, ownerID, doc.ID)
	}()

	// 2. Claim job from queue
	claimedJob, err := jobRepo.Claim(ctx, "e2e-worker")
	require.NoError(t, err)
	require.NotNil(t, claimedJob)
	assert.Equal(t, "running", claimedJob.Status)
	assert.Equal(t, "e2e-worker", *claimedJob.LockedBy)

	// 3. Process job through ingestion handler
	err = ingestHandler.HandleJob(ctx, claimedJob)
	require.NoError(t, err)

	// 4. Verify document transitioned to ready
	readyDoc, err := docRepo.GetByID(ctx, ownerID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", readyDoc.Status)
	assert.Nil(t, readyDoc.ErrorMessage)
	assert.Greater(t, readyDoc.ChunkCount, 0)

	// 5. Verify job status updated to succeeded
	finishedJob, err := jobRepo.GetByID(ctx, claimedJob.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", finishedJob.Status)

	// 6. Test similarity search and retrieval
	// Because FakeEmbedder derives deterministic vectors from exact sha256 text hashes,
	// querying with the chunk text yields an exact vector match (similarity score 1.0).
	var chunkText string
	err = pool.QueryRow(ctx, "SELECT content FROM chunks WHERE document_id = $1 ORDER BY chunk_index ASC LIMIT 1", doc.ID).Scan(&chunkText)
	require.NoError(t, err)

	retrievalSvc := service.NewRetrievalService(fakeEmbedder, chunkRepo, service.RetrievalConfig{
		TopK:     6,
		MinScore: 0.65,
	}, nil)

	retrievalRes, err := retrievalSvc.Retrieve(ctx, ownerID, chunkText, nil)
	require.NoError(t, err)
	assert.True(t, retrievalRes.HasRelevantContext)
	require.NotEmpty(t, retrievalRes.Matches)
	assert.Equal(t, "handbook.md", retrievalRes.Matches[0].DocumentName)
	assert.GreaterOrEqual(t, retrievalRes.TopScore, 0.99)

	// 7. Test RAG answering with fake LLM
	fakeLLM := llm.NewFakeLLM("Full-time employees receive 20 days of PTO per calendar year [1].")
	ragSvc := service.NewRAGService(retrievalSvc, fakeLLM, nil)

	answer, err := ragSvc.Ask(ctx, ownerID, chunkText, nil)
	require.NoError(t, err)
	assert.True(t, answer.Grounded)
	assert.Contains(t, answer.Answer, "20 days of PTO")
	require.NotEmpty(t, answer.Citations)
	assert.Equal(t, 1, answer.Citations[0].Marker)
	assert.Equal(t, "handbook.md", answer.Citations[0].DocumentName)
	assert.Equal(t, doc.ID, answer.Citations[0].DocumentID)
}
