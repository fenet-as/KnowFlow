package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
)

// JobHandler defines the contract for processing a specific job type.
type JobHandler interface {
	HandleJob(ctx context.Context, job *domain.Job) error
}

// JobHandlerFunc allows plain functions to satisfy JobHandler.
type JobHandlerFunc func(ctx context.Context, job *domain.Job) error

// HandleJob calls f(ctx, job).
func (f JobHandlerFunc) HandleJob(ctx context.Context, job *domain.Job) error {
	return f(ctx, job)
}

// PoolConfig holds the runtime options for background job processing.
type PoolConfig struct {
	WorkerCount        int
	PollInterval       time.Duration
	JobTimeout         time.Duration
	ShutdownTimeout    time.Duration
	StaleCheckInterval time.Duration
	JobStaleAfter      time.Duration
	Logger             *slog.Logger
}

// DefaultPoolConfig returns standard defaults for the worker pool.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		WorkerCount:        2,
		PollInterval:       1 * time.Second,
		JobTimeout:         5 * time.Minute,
		ShutdownTimeout:    10 * time.Second,
		StaleCheckInterval: 30 * time.Second,
		JobStaleAfter:      10 * time.Minute,
		Logger:             slog.Default(),
	}
}

// Pool manages a pool of worker goroutines claiming and executing async jobs,
// along with a periodic sweeper recovering stale running jobs (KF-23, KF-26).
type Pool struct {
	cfg      PoolConfig
	jobRepo  repo.JobRepo
	handlers map[string]JobHandler

	wg      sync.WaitGroup
	cancel  context.CancelFunc
	running atomic.Bool
	mu      sync.RWMutex
}

// NewPool constructs a new worker pool instance.
func NewPool(cfg PoolConfig, jobRepo repo.JobRepo) *Pool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 2
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 5 * time.Minute
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.StaleCheckInterval <= 0 {
		cfg.StaleCheckInterval = 30 * time.Second
	}
	if cfg.JobStaleAfter <= 0 {
		cfg.JobStaleAfter = 10 * time.Minute
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Pool{
		cfg:      cfg,
		jobRepo:  jobRepo,
		handlers: make(map[string]JobHandler),
	}
}

// RegisterHandler binds a job type name (e.g. "ingest_document") to a handler.
func (p *Pool) RegisterHandler(jobType string, handler JobHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[jobType] = handler
}

// Start launches the configured worker goroutines and the periodic stale job recovery sweeper.
func (p *Pool) Start(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		return errors.New("worker pool is already running")
	}

	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	instanceID := uuid.New().String()[:8]
	p.cfg.Logger.Info("Starting worker pool",
		"worker_count", p.cfg.WorkerCount,
		"poll_interval", p.cfg.PollInterval,
		"stale_check_interval", p.cfg.StaleCheckInterval,
		"job_stale_after", p.cfg.JobStaleAfter,
		"instance_id", instanceID,
	)

	// Launch worker goroutines
	for i := 0; i < p.cfg.WorkerCount; i++ {
		workerID := fmt.Sprintf("worker-%d-%s", i, instanceID)
		p.wg.Add(1)
		go p.workerLoop(runCtx, workerID)
	}

	// Launch background stale job recovery sweeper (KF-26)
	p.wg.Add(1)
	go p.staleSweeperLoop(runCtx)

	return nil
}

// workerLoop repeatedly polls for ready jobs and dispatches them to registered handlers.
func (p *Pool) workerLoop(ctx context.Context, workerID string) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := p.jobRepo.Claim(ctx, workerID)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				p.cfg.Logger.Error("Worker claim error", "worker_id", workerID, "error", err)
			}
			p.sleepJittered(ctx, p.cfg.PollInterval)
			continue
		}

		if job == nil {
			// No eligible jobs currently in queue
			p.sleepJittered(ctx, p.cfg.PollInterval)
			continue
		}

		p.processJob(ctx, workerID, job)
	}
}

// staleSweeperLoop periodically recovers stale running jobs whose heartbeat has timed out (KF-26).
func (p *Pool) staleSweeperLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cfg.StaleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			recovered, err := p.jobRepo.RecoverStale(ctx, p.cfg.JobStaleAfter)
			if err != nil && !errors.Is(err, context.Canceled) {
				p.cfg.Logger.Error("Stale job sweeper error", "error", err)
			} else if recovered > 0 {
				p.cfg.Logger.Info("Recovered stale running jobs back to queued",
					"count", recovered,
					"stale_after", p.cfg.JobStaleAfter,
				)
			}
		}
	}
}

// processJob executes the job handler with panic recovery and timeout protection.
func (p *Pool) processJob(parentCtx context.Context, workerID string, job *domain.Job) {
	p.mu.RLock()
	handler, exists := p.handlers[job.Type]
	p.mu.RUnlock()

	if !exists {
		errMsg := fmt.Sprintf("no handler registered for job type: %s", job.Type)
		p.cfg.Logger.Error(errMsg, "job_id", job.ID, "type", job.Type, "worker_id", workerID)
		failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.jobRepo.Fail(failCtx, job.ID, errMsg)
		return
	}

	// Panic recovery guarantees no uncaught panic crashes the process or leaves a job locked
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic in job handler: %v", r)
			p.cfg.Logger.Error("Job panicked in handler",
				"job_id", job.ID,
				"type", job.Type,
				"worker_id", workerID,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = p.jobRepo.Fail(failCtx, job.ID, errMsg)
		}
	}()

	jobTimeout := p.cfg.JobTimeout
	jobCtx, jobCancel := context.WithTimeout(parentCtx, jobTimeout)
	defer jobCancel()

	if err := handler.HandleJob(jobCtx, job); err != nil {
		p.cfg.Logger.Warn("Job execution failed",
			"job_id", job.ID,
			"type", job.Type,
			"worker_id", workerID,
			"error", err,
		)
	}
}

// sleepJittered sleeps for baseInterval plus a small pseudo-random jitter.
func (p *Pool) sleepJittered(ctx context.Context, base time.Duration) {
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	// Jitter: 0 to 25% of base interval
	jitterMagnitude := int64(base / 4)
	var jitter time.Duration
	if jitterMagnitude > 0 {
		jitter = time.Duration(rand.Int63n(jitterMagnitude))
	}

	timer := time.NewTimer(base + jitter)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// Shutdown gracefully stops the worker pool by ceasing new claims and waiting
// for all in-flight jobs to complete up to the given context's deadline.
func (p *Pool) Shutdown(ctx context.Context) error {
	if !p.running.CompareAndSwap(true, false) {
		return nil
	}

	p.cfg.Logger.Info("Stopping worker pool, waiting for in-flight jobs to complete")

	// Cancel context to stop workers and sweeper
	if p.cancel != nil {
		p.cancel()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.cfg.Logger.Info("All worker pool goroutines finished cleanly")
		return nil
	case <-ctx.Done():
		p.cfg.Logger.Warn("Worker pool shutdown deadline exceeded", "error", ctx.Err())
		return ctx.Err()
	}
}

// Stop gracefully shuts down the worker pool using the configured ShutdownTimeout.
func (p *Pool) Stop() {
	timeout := p.cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = p.Shutdown(ctx)
}
