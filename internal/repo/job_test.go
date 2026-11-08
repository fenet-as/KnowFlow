package repo_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupJobs(t *testing.T, ctx context.Context, pool repoDBPool) {
	_, err := pool.Exec(ctx, "DELETE FROM jobs")
	require.NoError(t, err)
}

type repoDBPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (any, error)
}

func TestPostgresJobRepo_Lifecycle(t *testing.T) {
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

	_, err = pool.Exec(ctx, "DELETE FROM jobs")
	require.NoError(t, err)

	docRepo := repo.NewPostgresDocumentRepo(pool)
	jobRepo := repo.NewPostgresJobRepo(pool)

	ownerID := "owner_job_test_" + uuid.New().String()[:8]

	// 1. Create a parent document
	doc := &domain.Document{
		OwnerID:     ownerID,
		Filename:    "job_test_doc.txt",
		SizeBytes:   128,
		ContentHash: "hash_" + uuid.New().String(),
		RawContent:  "Sample raw content for job testing",
	}
	initialJob, err := docRepo.CreateWithJob(ctx, doc, 5)
	require.NoError(t, err)
	require.NotNil(t, initialJob)

	// Clean up initial job first to have clean slate for this doc
	err = jobRepo.Complete(ctx, initialJob.ID)
	require.NoError(t, err)

	// 2. Test Enqueue without tx
	job1, err := jobRepo.Enqueue(ctx, nil, doc.ID, 3)
	require.NoError(t, err)
	require.NotNil(t, job1)
	assert.Equal(t, doc.ID, job1.DocumentID)
	assert.Equal(t, "queued", job1.Status)
	assert.Equal(t, 0, job1.Attempts)
	assert.Equal(t, 3, job1.MaxAttempts)

	// 3. Test Enqueue with external tx
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	job2, err := jobRepo.Enqueue(ctx, tx, doc.ID, 5)
	require.NoError(t, err)
	require.NotNil(t, job2)
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// 4. Test Claim: Worker 1 claims job1
	claimedJob, err := jobRepo.Claim(ctx, "worker-node-1")
	require.NoError(t, err)
	require.NotNil(t, claimedJob)
	assert.Equal(t, "running", claimedJob.Status)
	assert.Equal(t, 1, claimedJob.Attempts)
	require.NotNil(t, claimedJob.LockedBy)
	assert.Equal(t, "worker-node-1", *claimedJob.LockedBy)
	require.NotNil(t, claimedJob.LockedAt)

	// 5. Test Complete
	err = jobRepo.Complete(ctx, claimedJob.ID)
	require.NoError(t, err)

	completedJob, err := jobRepo.GetByID(ctx, claimedJob.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", completedJob.Status)
	assert.Nil(t, completedJob.LockedBy)
	assert.Nil(t, completedJob.LockedAt)

	// 6. Test RetryLater and Fail on job2
	claimedJob2, err := jobRepo.Claim(ctx, "worker-node-2")
	require.NoError(t, err)
	require.NotNil(t, claimedJob2)
	assert.Equal(t, job2.ID, claimedJob2.ID)

	err = jobRepo.RetryLater(ctx, claimedJob2.ID, 100*time.Millisecond, "rate limit exceeded")
	require.NoError(t, err)

	retriedJob, err := jobRepo.GetByID(ctx, claimedJob2.ID)
	require.NoError(t, err)
	assert.Equal(t, "queued", retriedJob.Status)
	assert.Nil(t, retriedJob.LockedBy)
	assert.Nil(t, retriedJob.LockedAt)
	require.NotNil(t, retriedJob.LastError)
	assert.Equal(t, "rate limit exceeded", *retriedJob.LastError)

	// Should not be claimable immediately due to run_after
	immediateClaim, err := jobRepo.Claim(ctx, "worker-node-3")
	require.NoError(t, err)
	assert.Nil(t, immediateClaim)

	// Wait past delay
	time.Sleep(150 * time.Millisecond)

	// Claim again after delay
	claimedAgain, err := jobRepo.Claim(ctx, "worker-node-3")
	require.NoError(t, err)
	require.NotNil(t, claimedAgain)
	assert.Equal(t, claimedJob2.ID, claimedAgain.ID)
	assert.Equal(t, 2, claimedAgain.Attempts)

	// Now fail it permanently
	err = jobRepo.Fail(ctx, claimedAgain.ID, "unrecoverable model error")
	require.NoError(t, err)

	failedJob, err := jobRepo.GetByID(ctx, claimedAgain.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", failedJob.Status)
	assert.Nil(t, failedJob.LockedBy)
	assert.Nil(t, failedJob.LockedAt)
	require.NotNil(t, failedJob.LastError)
	assert.Equal(t, "unrecoverable model error", *failedJob.LastError)

	// No more queued jobs
	noJob, err := jobRepo.Claim(ctx, "worker-node-1")
	require.NoError(t, err)
	assert.Nil(t, noJob)
}

func TestPostgresJobRepo_RecoverStale(t *testing.T) {
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

	_, err = pool.Exec(ctx, "DELETE FROM jobs")
	require.NoError(t, err)

	docRepo := repo.NewPostgresDocumentRepo(pool)
	jobRepo := repo.NewPostgresJobRepo(pool)

	ownerID := "owner_stale_" + uuid.New().String()[:8]
	doc := &domain.Document{
		OwnerID:     ownerID,
		Filename:    "stale_test_doc.txt",
		SizeBytes:   50,
		ContentHash: "hash_" + uuid.New().String(),
		RawContent:  "Stale recovery test",
	}
	_, err = docRepo.CreateWithJob(ctx, doc, 5)
	require.NoError(t, err)

	// Claim the job to make it 'running'
	claimed, err := jobRepo.Claim(ctx, "crashed-worker")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, "running", claimed.Status)

	// Stale recovery with 1 hour duration should NOT recover it yet
	recovered, err := jobRepo.RecoverStale(ctx, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), recovered)

	// Manually set locked_at to 10 seconds ago to simulate a stale/crashed worker
	_, err = pool.Exec(ctx, "UPDATE jobs SET locked_at = now() - interval '10 seconds' WHERE id = $1", claimed.ID)
	require.NoError(t, err)

	// Now run RecoverStale with 5 second threshold
	recovered, err = jobRepo.RecoverStale(ctx, 5*time.Second)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, recovered, int64(1))

	// Verify job is back in queued status with locks cleared
	recoveredJob, err := jobRepo.GetByID(ctx, claimed.ID)
	require.NoError(t, err)
	assert.Equal(t, "queued", recoveredJob.Status)
	assert.Nil(t, recoveredJob.LockedAt)
	assert.Nil(t, recoveredJob.LockedBy)

	// Clean up by claiming and completing
	cleaned, err := jobRepo.Claim(ctx, "cleaner-worker")
	require.NoError(t, err)
	require.NotNil(t, cleaned)
	err = jobRepo.Complete(ctx, cleaned.ID)
	require.NoError(t, err)
}

func TestPostgresJobRepo_ConcurrentClaimers(t *testing.T) {
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

	_, err = pool.Exec(ctx, "DELETE FROM jobs")
	require.NoError(t, err)

	docRepo := repo.NewPostgresDocumentRepo(pool)
	jobRepo := repo.NewPostgresJobRepo(pool)

	ownerID := "owner_concurrent_" + uuid.New().String()[:8]
	doc := &domain.Document{
		OwnerID:     ownerID,
		Filename:    "concurrent_test_doc.txt",
		SizeBytes:   200,
		ContentHash: "hash_" + uuid.New().String(),
		RawContent:  "Concurrent claiming test",
	}
	// Initial job from doc creation
	initJob, err := docRepo.CreateWithJob(ctx, doc, 5)
	require.NoError(t, err)
	err = jobRepo.Complete(ctx, initJob.ID)
	require.NoError(t, err)

	// Enqueue 15 distinct jobs
	const totalJobs = 15
	jobIDs := make([]uuid.UUID, totalJobs)
	for i := 0; i < totalJobs; i++ {
		j, err := jobRepo.Enqueue(ctx, nil, doc.ID, 5)
		require.NoError(t, err)
		jobIDs[i] = j.ID
	}

	// 5 concurrent workers race to claim all jobs
	const numWorkers = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedMap := make(map[uuid.UUID]int)
	workerClaims := make(map[string]int)

	for w := 0; w < numWorkers; w++ {
		workerID := fmt.Sprintf("worker-%d", w)
		wg.Add(1)
		go func(wid string) {
			defer wg.Done()
			for {
				job, err := jobRepo.Claim(ctx, wid)
				if err != nil {
					t.Errorf("worker %s error: %v", wid, err)
					return
				}
				if job == nil {
					// All currently available jobs claimed
					return
				}

				mu.Lock()
				claimedMap[job.ID]++
				workerClaims[wid]++
				mu.Unlock()

				// Complete job immediately
				_ = jobRepo.Complete(ctx, job.ID)
			}
		}(workerID)
	}

	wg.Wait()

	// Verify that every job was claimed exactly once
	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, totalJobs, len(claimedMap), "every job should be claimed")
	for _, jID := range jobIDs {
		assert.Equal(t, 1, claimedMap[jID], "job %s must be claimed exactly once", jID)
	}

	t.Logf("Worker claim distribution across %d workers: %v", numWorkers, workerClaims)
}
