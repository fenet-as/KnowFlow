package worker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockJobRepo struct {
	mu           sync.Mutex
	jobs         []*domain.Job
	claimedJobs  []*domain.Job
	failedJobs   map[uuid.UUID]string
	claimCount   atomic.Int32
	claimBlocker chan struct{}
}

func newMockJobRepo() *mockJobRepo {
	return &mockJobRepo{
		failedJobs: make(map[uuid.UUID]string),
	}
}

func (m *mockJobRepo) Enqueue(ctx context.Context, tx pgx.Tx, documentID uuid.UUID, maxAttempts int) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := &domain.Job{
		ID:          uuid.New(),
		DocumentID:  documentID,
		Type:        "test_job",
		Status:      "queued",
		MaxAttempts: maxAttempts,
		RunAfter:    time.Now(),
		CreatedAt:   time.Now(),
	}
	m.jobs = append(m.jobs, j)
	return j, nil
}

func (m *mockJobRepo) Claim(ctx context.Context, workerID string) (*domain.Job, error) {
	if m.claimBlocker != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.claimBlocker:
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimCount.Add(1)

	for _, j := range m.jobs {
		if j.Status == "queued" {
			j.Status = "running"
			j.Attempts++
			j.LockedBy = &workerID
			now := time.Now()
			j.LockedAt = &now
			m.claimedJobs = append(m.claimedJobs, j)
			return j, nil
		}
	}
	return nil, nil
}

func (m *mockJobRepo) Complete(ctx context.Context, jobID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == jobID {
			j.Status = "succeeded"
			j.LockedAt = nil
			j.LockedBy = nil
			return nil
		}
	}
	return errors.New("job not found")
}

func (m *mockJobRepo) RetryLater(ctx context.Context, jobID uuid.UUID, delay time.Duration, lastError string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == jobID {
			j.Status = "queued"
			j.RunAfter = time.Now().Add(delay)
			j.LastError = &lastError
			j.LockedAt = nil
			j.LockedBy = nil
			return nil
		}
	}
	return errors.New("job not found")
}

func (m *mockJobRepo) Fail(ctx context.Context, jobID uuid.UUID, lastError string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedJobs[jobID] = lastError
	for _, j := range m.jobs {
		if j.ID == jobID {
			j.Status = "failed"
			j.LastError = &lastError
			j.LockedAt = nil
			j.LockedBy = nil
			return nil
		}
	}
	return nil
}

func (m *mockJobRepo) RecoverStale(ctx context.Context, staleDuration time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockJobRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return nil, domain.NewNotFoundError("Job not found.")
}

func TestWorkerPool_LifecycleAndProcessing(t *testing.T) {
	repo := newMockJobRepo()
	_, _ = repo.Enqueue(context.Background(), nil, uuid.New(), 5)
	_, _ = repo.Enqueue(context.Background(), nil, uuid.New(), 5)

	var handledCount atomic.Int32

	handler := worker.JobHandlerFunc(func(ctx context.Context, job *domain.Job) error {
		handledCount.Add(1)
		return repo.Complete(ctx, job.ID)
	})

	cfg := worker.PoolConfig{
		WorkerCount:     2,
		PollInterval:    20 * time.Millisecond,
		JobTimeout:      2 * time.Second,
		ShutdownTimeout: 2 * time.Second,
	}

	pool := worker.NewPool(cfg, repo)
	pool.RegisterHandler("test_job", handler)

	err := pool.Start(context.Background())
	require.NoError(t, err)

	// Wait for jobs to be processed
	assert.Eventually(t, func() bool {
		return handledCount.Load() == 2
	}, 1*time.Second, 20*time.Millisecond)

	pool.Stop()
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	repo := newMockJobRepo()
	job, _ := repo.Enqueue(context.Background(), nil, uuid.New(), 5)

	panickingHandler := worker.JobHandlerFunc(func(ctx context.Context, j *domain.Job) error {
		panic("fatal unexpected nil pointer")
	})

	cfg := worker.PoolConfig{
		WorkerCount:     1,
		PollInterval:    20 * time.Millisecond,
		JobTimeout:      1 * time.Second,
		ShutdownTimeout: 1 * time.Second,
	}

	pool := worker.NewPool(cfg, repo)
	pool.RegisterHandler("test_job", panickingHandler)

	err := pool.Start(context.Background())
	require.NoError(t, err)

	// Assert that job is marked failed in repo and pool did not crash
	assert.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		_, failed := repo.failedJobs[job.ID]
		return failed
	}, 1*time.Second, 20*time.Millisecond)

	repo.mu.Lock()
	errMsg := repo.failedJobs[job.ID]
	repo.mu.Unlock()
	assert.Contains(t, errMsg, "panic in job handler: fatal unexpected nil pointer")

	pool.Stop()
}

func TestWorkerPool_GracefulShutdownWaitsForInFlightJobs(t *testing.T) {
	repo := newMockJobRepo()
	job, _ := repo.Enqueue(context.Background(), nil, uuid.New(), 5)

	jobStarted := make(chan struct{})
	jobFinished := make(chan struct{})

	slowHandler := worker.JobHandlerFunc(func(ctx context.Context, j *domain.Job) error {
		close(jobStarted)
		time.Sleep(100 * time.Millisecond)
		close(jobFinished)
		return repo.Complete(ctx, j.ID)
	})

	cfg := worker.PoolConfig{
		WorkerCount:     1,
		PollInterval:    20 * time.Millisecond,
		JobTimeout:      2 * time.Second,
		ShutdownTimeout: 2 * time.Second,
	}

	pool := worker.NewPool(cfg, repo)
	pool.RegisterHandler("test_job", slowHandler)

	err := pool.Start(context.Background())
	require.NoError(t, err)

	// Wait until handler is actively in-flight
	<-jobStarted

	// Call Shutdown with sufficient timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = pool.Shutdown(shutdownCtx)
	require.NoError(t, err)

	// Verify that in-flight job completed cleanly before shutdown returned
	select {
	case <-jobFinished:
		// Success
	default:
		t.Fatal("expected in-flight job to finish before shutdown completed")
	}

	j, err := repo.GetByID(context.Background(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", j.Status)
}
