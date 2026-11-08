package repo

import (
	"context"
	"errors"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresJobRepo implements JobRepo using pgxpool and PostgreSQL transactional queuing.
type PostgresJobRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresJobRepo constructs a PostgresJobRepo.
func NewPostgresJobRepo(pool *pgxpool.Pool) *PostgresJobRepo {
	return &PostgresJobRepo{pool: pool}
}

// Enqueue inserts a new background ingestion job into the jobs table.
// If an external transaction is provided via tx, the insert participates in that transaction.
func (r *PostgresJobRepo) Enqueue(
	ctx context.Context,
	tx pgx.Tx,
	documentID uuid.UUID,
	maxAttempts int,
) (*domain.Job, error) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	insertSQL := `
		INSERT INTO jobs (
			document_id, type, status, attempts, max_attempts, run_after
		) VALUES ($1, 'ingest_document', 'queued', 0, $2, now())
		RETURNING id, document_id, type, status, attempts, max_attempts, run_after, locked_at, locked_by, last_error, created_at;
	`

	job := &domain.Job{}
	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, insertSQL, documentID, maxAttempts)
	} else {
		row = r.pool.QueryRow(ctx, insertSQL, documentID, maxAttempts)
	}

	err := row.Scan(
		&job.ID,
		&job.DocumentID,
		&job.Type,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.RunAfter,
		&job.LockedAt,
		&job.LockedBy,
		&job.LastError,
		&job.CreatedAt,
	)
	if err != nil {
		return nil, domain.NewInternalError("failed to enqueue job", err)
	}

	return job, nil
}

// Claim selects the next available queued job using FOR UPDATE SKIP LOCKED,
// increments its attempt count, sets its status to 'running', and marks it locked by workerID.
// If no jobs are eligible, it returns (nil, nil).
func (r *PostgresJobRepo) Claim(ctx context.Context, workerID string) (*domain.Job, error) {
	if workerID == "" {
		return nil, domain.NewValidationError("workerID is required to claim a job", nil)
	}

	query := `
		WITH next AS (
			SELECT id
			FROM jobs
			WHERE status = 'queued'
			  AND run_after <= now()
			ORDER BY run_after ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE jobs j
		SET status = 'running',
		    locked_at = now(),
		    locked_by = $1,
		    attempts = j.attempts + 1
		FROM next
		WHERE j.id = next.id
		RETURNING j.id, j.document_id, j.type, j.status, j.attempts, j.max_attempts, j.run_after, j.locked_at, j.locked_by, j.last_error, j.created_at;
	`

	job := &domain.Job{}
	err := r.pool.QueryRow(ctx, query, workerID).Scan(
		&job.ID,
		&job.DocumentID,
		&job.Type,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.RunAfter,
		&job.LockedAt,
		&job.LockedBy,
		&job.LastError,
		&job.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, domain.NewInternalError("failed to claim job", err)
	}

	return job, nil
}

// Complete marks a job as 'succeeded' and clears the lock fields.
func (r *PostgresJobRepo) Complete(ctx context.Context, jobID uuid.UUID) error {
	query := `
		UPDATE jobs
		SET status = 'succeeded',
		    locked_at = NULL,
		    locked_by = NULL
		WHERE id = $1;
	`

	tag, err := r.pool.Exec(ctx, query, jobID)
	if err != nil {
		return domain.NewInternalError("failed to complete job", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("Job not found.")
	}
	return nil
}

// RetryLater schedules a job to be retried after delay, marking it back to 'queued'
// and recording lastError while clearing current worker lock state.
func (r *PostgresJobRepo) RetryLater(ctx context.Context, jobID uuid.UUID, delay time.Duration, lastError string) error {
	query := `
		UPDATE jobs
		SET status = 'queued',
		    run_after = now() + ($2::float8 * interval '1 second'),
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = $3
		WHERE id = $1;
	`

	tag, err := r.pool.Exec(ctx, query, jobID, delay.Seconds(), lastError)
	if err != nil {
		return domain.NewInternalError("failed to reschedule job for retry", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("Job not found.")
	}
	return nil
}

// Fail permanently marks a job as 'failed', recording the final error message and clearing locks.
func (r *PostgresJobRepo) Fail(ctx context.Context, jobID uuid.UUID, lastError string) error {
	query := `
		UPDATE jobs
		SET status = 'failed',
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = $2
		WHERE id = $1;
	`

	tag, err := r.pool.Exec(ctx, query, jobID, lastError)
	if err != nil {
		return domain.NewInternalError("failed to mark job as failed", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("Job not found.")
	}
	return nil
}

// RecoverStale identifies running jobs whose locked_at timestamp is older than staleDuration
// and resets them to 'queued' so they can be reclaimed.
func (r *PostgresJobRepo) RecoverStale(ctx context.Context, staleDuration time.Duration) (int64, error) {
	query := `
		UPDATE jobs
		SET status = 'queued',
		    locked_at = NULL,
		    locked_by = NULL
		WHERE status = 'running'
		  AND locked_at < now() - ($1::float8 * interval '1 second');
	`

	tag, err := r.pool.Exec(ctx, query, staleDuration.Seconds())
	if err != nil {
		return 0, domain.NewInternalError("failed to recover stale jobs", err)
	}
	return tag.RowsAffected(), nil
}

// GetByID returns the job by ID.
func (r *PostgresJobRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	query := `
		SELECT id, document_id, type, status, attempts, max_attempts, run_after, locked_at, locked_by, last_error, created_at
		FROM jobs
		WHERE id = $1;
	`

	job := &domain.Job{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.DocumentID,
		&job.Type,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.RunAfter,
		&job.LockedAt,
		&job.LockedBy,
		&job.LastError,
		&job.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("Job not found.")
		}
		return nil, domain.NewInternalError("failed to get job by id", err)
	}
	return job, nil
}
