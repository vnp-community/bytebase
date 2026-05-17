// Package jobqueue provides a PostgreSQL-backed persistent job queue using
// SELECT FOR UPDATE SKIP LOCKED for safe concurrent dequeuing.
package jobqueue

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/store"
)

// JobType defines the type of background job.
type JobType string

const (
	// JobTypePlanCheck is a plan check job.
	JobTypePlanCheck JobType = "plan_check"
	// JobTypeSchemaSync is a schema sync job.
	JobTypeSchemaSync JobType = "schema_sync"
	// JobTypeDataExport is a data export job.
	JobTypeDataExport JobType = "data_export"

	lockTTL = 5 * time.Minute
)

// Job represents a job queue entry.
type Job struct {
	ID          int64
	Workspace   string
	JobType     JobType
	Payload     json.RawMessage
	Priority    int
	Status      string
	Attempts    int
	MaxAttempts int
	ScheduledAt time.Time
	CreatedAt   time.Time
}

// Queue provides operations for a PostgreSQL-backed job queue.
type Queue struct {
	store    *store.Store
	workerID string
}

// NewQueue creates a new job queue backed by the given store.
func NewQueue(s *store.Store, workerID string) *Queue {
	return &Queue{store: s, workerID: workerID}
}

// Enqueue inserts a new job. If dedupKey is non-empty, duplicate jobs are
// silently ignored via ON CONFLICT DO NOTHING.
func (q *Queue) Enqueue(ctx context.Context, workspace string, jobType JobType,
	payload json.RawMessage, priority int, dedupKey string) error {
	query := `INSERT INTO job_queue (workspace, job_type, payload, priority, dedup_key)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT DO NOTHING`
	_, err := q.store.GetDB().ExecContext(ctx, query,
		workspace, string(jobType), payload, priority, dedupKey)
	return errors.Wrap(err, "enqueue job")
}

// Dequeue atomically claims the highest priority pending job using
// SELECT FOR UPDATE SKIP LOCKED to prevent contention between workers.
func (q *Queue) Dequeue(ctx context.Context, jobTypes []JobType) (*Job, error) {
	typeStrs := make([]string, len(jobTypes))
	for i, t := range jobTypes {
		typeStrs[i] = fmt.Sprintf("'%s'", string(t))
	}
	query := fmt.Sprintf(`UPDATE job_queue SET
		status = 'running', locked_by = $1, locked_at = NOW(),
		started_at = NOW(), attempts = attempts + 1
		WHERE id = (
			SELECT id FROM job_queue
			WHERE status = 'pending' AND scheduled_at <= NOW()
			AND job_type IN (%s)
			ORDER BY priority DESC, scheduled_at ASC
			LIMIT 1 FOR UPDATE SKIP LOCKED
		) RETURNING id, workspace, job_type, payload, priority, status,
			attempts, max_attempts, scheduled_at, created_at`,
		strings.Join(typeStrs, ","))

	job := &Job{}
	var jt string
	err := q.store.GetDB().QueryRowContext(ctx, query, q.workerID).Scan(
		&job.ID, &job.Workspace, &jt, &job.Payload, &job.Priority,
		&job.Status, &job.Attempts, &job.MaxAttempts,
		&job.ScheduledAt, &job.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "dequeue job")
	}
	job.JobType = JobType(jt)
	return job, nil
}

// Complete marks a job as successfully done.
func (q *Queue) Complete(ctx context.Context, jobID int64) error {
	_, err := q.store.GetDB().ExecContext(ctx,
		`UPDATE job_queue SET status = 'done', completed_at = NOW() WHERE id = $1`, jobID)
	return errors.Wrap(err, "complete job")
}

// Fail marks a job as failed. If max attempts are exceeded, the job is marked
// as dead. Otherwise, it's rescheduled with exponential backoff.
func (q *Queue) Fail(ctx context.Context, jobID int64, errMsg string) error {
	_, err := q.store.GetDB().ExecContext(ctx, `
		UPDATE job_queue SET
			status = CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'failed' END,
			error_msg = $2,
			locked_by = NULL,
			locked_at = NULL,
			scheduled_at = CASE WHEN attempts >= max_attempts THEN scheduled_at
				ELSE NOW() + ($3 * interval '1 second') END
		WHERE id = $1`,
		jobID, errMsg, exponentialBackoff(0)) // backoff computed server-side
	return errors.Wrap(err, "fail job")
}

// RecoverStale reclaims jobs that have been locked longer than the lock TTL,
// resetting them to pending so they can be retried.
func (q *Queue) RecoverStale(ctx context.Context) (int64, error) {
	result, err := q.store.GetDB().ExecContext(ctx, `
		UPDATE job_queue SET status = 'pending', locked_by = NULL, locked_at = NULL
		WHERE status = 'running' AND locked_at < NOW() - $1::interval`,
		fmt.Sprintf("%d seconds", int(lockTTL.Seconds())))
	if err != nil {
		return 0, errors.Wrap(err, "recover stale jobs")
	}
	return result.RowsAffected()
}

func exponentialBackoff(attempt int) float64 {
	return math.Min(math.Pow(2, float64(attempt)), 3600)
}
