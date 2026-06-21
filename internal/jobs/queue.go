package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type SQLQueue struct {
	db       DB
	newJobID func() string
}

func NewSQLQueue(db DB, newJobID func() string) *SQLQueue {
	return &SQLQueue{
		db:       db,
		newJobID: newJobID,
	}
}

func (q *SQLQueue) Enqueue(ctx context.Context, job Job) error {
	if job.ID == "" {
		if q.newJobID != nil {
			job.ID = q.newJobID()
		} else {
			return fmt.Errorf("job ID is required")
		}
	}
	if job.Status == "" {
		job.Status = StatusQueued
	}
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	if job.RunAfter.IsZero() {
		job.RunAfter = time.Now()
	}

	query := `
		INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_after, locked_by, locked_at, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			payload = EXCLUDED.payload,
			status = EXCLUDED.status,
			attempts = EXCLUDED.attempts,
			max_attempts = EXCLUDED.max_attempts,
			run_after = EXCLUDED.run_after,
			locked_by = EXCLUDED.locked_by,
			locked_at = EXCLUDED.locked_at,
			last_error = EXCLUDED.last_error,
			updated_at = NOW()
	`
	var lastErr sql.NullString
	if job.LastError != "" {
		lastErr = sql.NullString{String: job.LastError, Valid: true}
	}
	var lockedAt sql.NullTime
	if !job.LockedAt.IsZero() {
		lockedAt = sql.NullTime{Time: job.LockedAt, Valid: true}
	}

	_, err := q.db.ExecContext(ctx, query, job.ID, job.Kind, job.Payload, string(job.Status), job.Attempts, job.MaxAttempts, job.RunAfter, job.LockedBy, lockedAt, lastErr)
	return err
}

func (q *SQLQueue) Claim(ctx context.Context, workerID string, limit int) ([]Job, error) {
	// Claim jobs using SELECT ... FOR UPDATE SKIP LOCKED to prevent race conditions.
	query := `
		UPDATE jobs
		SET status = 'running',
			attempts = attempts + 1,
			locked_by = $2,
			locked_at = NOW(),
			updated_at = NOW()
		WHERE id IN (
			SELECT id
			FROM jobs
			WHERE status = 'queued'
			  AND run_after <= NOW()
			  AND attempts < max_attempts
			ORDER BY run_after ASC, id ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, kind, payload, status, attempts, max_attempts, run_after, locked_by, locked_at, last_error
	`

	rows, err := q.db.QueryContext(ctx, query, limit, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var statusStr string
		var lastErr sql.NullString
		var lockedBy sql.NullString
		var lockedAt sql.NullTime
		err := rows.Scan(&job.ID, &job.Kind, &job.Payload, &statusStr, &job.Attempts, &job.MaxAttempts, &job.RunAfter, &lockedBy, &lockedAt, &lastErr)
		if err != nil {
			return nil, err
		}
		job.Status = Status(statusStr)
		if lockedBy.Valid {
			job.LockedBy = lockedBy.String
		}
		if lockedAt.Valid {
			job.LockedAt = lockedAt.Time
		}
		if lastErr.Valid {
			job.LastError = lastErr.String
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (q *SQLQueue) Complete(ctx context.Context, id string) error {
	query := `
		UPDATE jobs
		SET status = 'succeeded',
			locked_by = NULL,
			locked_at = NULL,
			last_error = NULL,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := q.db.ExecContext(ctx, query, id)
	return err
}

func (q *SQLQueue) Fail(ctx context.Context, id string, cause error) error {
	query := `
		UPDATE jobs
		SET status = CASE WHEN attempts < max_attempts THEN 'queued' ELSE 'failed' END,
			last_error = $2,
			run_after = CASE WHEN attempts < max_attempts THEN NOW() + make_interval(secs => LEAST(300, 5 * attempts)) ELSE run_after END,
			locked_by = NULL,
			locked_at = NULL,
			updated_at = NOW()
		WHERE id = $1
	`
	var errStr string
	if cause != nil {
		errStr = cause.Error()
	}
	_, err := q.db.ExecContext(ctx, query, id, errStr)
	return err
}

func (q *SQLQueue) Cancel(ctx context.Context, id string) (bool, error) {
	result, err := q.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'cancelled',
			locked_by = NULL,
			locked_at = NULL,
			updated_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'failed')
	`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
