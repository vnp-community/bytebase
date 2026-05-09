# Solution: CR-PERF-006 — Runner Orchestration & Job Queue

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-006                              |
| **Solution ID**| SOL-PERF-006                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L5 (Bus), L6 (Runner), L8 (Store)        |
| **TDD Refs**   | §5.1 Message Bus, §5.2 Task Execution, §5.4 LISTEN/NOTIFY |

---

## 1. Solution Overview

Từ TDD §5.1, Bus sử dụng buffered Go channels (1000 buffer). Messages mất khi crash. Với 200K databases, runners cạnh tranh CPU/connections.

**Approach**: Không thay toàn bộ Bus — giữ channels cho real-time coordination (low latency), thêm **PostgreSQL-based job queue** cho durable, distributable tasks. Tận dụng existing `advisory_lock.go` (TDD §5.4) và PG LISTEN/NOTIFY cho inter-replica coordination.

```
Bus (channels) → Real-time signals (keep)
Job Queue (PG) → Durable task processing (new)
```

---

## 2. Detailed Technical Design

### 2.1 Job Queue Table

**File**: `backend/migrator/migration/<next_version>/0003_job_queue.sql`

```sql
CREATE TABLE IF NOT EXISTS job_queue (
    id          BIGSERIAL PRIMARY KEY,
    workspace   TEXT NOT NULL,
    job_type    TEXT NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    priority    INT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'running', 'done', 'failed', 'dead')),
    locked_by   TEXT,                -- server instance ID
    locked_at   TIMESTAMPTZ,
    attempts    INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at  TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_msg   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Deduplication key to prevent duplicate enqueue
    dedup_key   TEXT,
    UNIQUE (dedup_key) WHERE dedup_key IS NOT NULL
);

-- Primary query: fetch pending jobs by priority
CREATE INDEX idx_job_queue_pending
    ON job_queue (priority DESC, scheduled_at ASC)
    WHERE status = 'pending';

-- Tenant query: pending jobs per workspace
CREATE INDEX idx_job_queue_workspace_pending
    ON job_queue (workspace, status)
    WHERE status IN ('pending', 'running');

-- Stale lock detection
CREATE INDEX idx_job_queue_locked
    ON job_queue (locked_at)
    WHERE status = 'running';

-- Notify workers when new job is enqueued
CREATE OR REPLACE FUNCTION notify_job_queue()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('bytebase:job_queue', NEW.job_type);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_job_queue_notify
    AFTER INSERT ON job_queue
    FOR EACH ROW EXECUTE FUNCTION notify_job_queue();
```

### 2.2 Job Queue Manager

**File**: `backend/component/jobqueue/queue.go` (new)

```go
package jobqueue

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"

    "github.com/bytebase/bytebase/backend/common/qb"
)

// JobType defines the type of background job.
type JobType string

const (
    JobTypePlanCheck  JobType = "plan_check"
    JobTypeApproval   JobType = "approval"
    JobTypeTaskRun    JobType = "task_run"
    JobTypeCleanup    JobType = "cleanup"
)

// Job represents a queued job.
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

// Queue provides durable job queue operations backed by PostgreSQL.
type Queue struct {
    db         *sql.DB
    instanceID string // This server's unique instance ID
    lockTTL    time.Duration
}

func New(db *sql.DB, instanceID string) *Queue {
    return &Queue{
        db:         db,
        instanceID: instanceID,
        lockTTL:    5 * time.Minute,
    }
}

// Enqueue adds a job to the queue. Uses dedup_key to prevent duplicates.
func (q *Queue) Enqueue(ctx context.Context, workspace string, jobType JobType,
    payload interface{}, priority int, dedupKey string) (int64, error) {

    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return 0, err
    }

    query := qb.Q().Space(`
        INSERT INTO job_queue (workspace, job_type, payload, priority, dedup_key)
        VALUES (?, ?, ?::jsonb, ?, ?)
        ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING
        RETURNING id`,
        workspace, string(jobType), string(payloadJSON), priority, dedupKey)

    sql, args, err := query.ToSQL()
    if err != nil {
        return 0, err
    }

    var id int64
    err = q.db.QueryRowContext(ctx, sql, args...).Scan(&id)
    if err != nil {
        return 0, err
    }
    return id, nil
}

// Dequeue fetches the next available job, atomically locking it.
// Uses SELECT FOR UPDATE SKIP LOCKED for concurrent safety.
func (q *Queue) Dequeue(ctx context.Context, jobTypes []JobType) (*Job, error) {
    types := make([]interface{}, len(jobTypes))
    placeholders := ""
    for i, jt := range jobTypes {
        types[i] = string(jt)
        if i > 0 { placeholders += "," }
        placeholders += "?"
    }

    query := qb.Q().Space(fmt.Sprintf(`
        UPDATE job_queue SET
            status = 'running',
            locked_by = ?,
            locked_at = NOW(),
            started_at = NOW(),
            attempts = attempts + 1
        WHERE id = (
            SELECT id FROM job_queue
            WHERE status = 'pending'
              AND scheduled_at <= NOW()
              AND job_type IN (%s)
            ORDER BY priority DESC, scheduled_at ASC
            LIMIT 1
            FOR UPDATE SKIP LOCKED
        )
        RETURNING id, workspace, job_type, payload, priority,
                  status, attempts, max_attempts, scheduled_at, created_at`,
        placeholders), append([]interface{}{q.instanceID}, types...)...)

    sql, args, err := query.ToSQL()
    if err != nil {
        return nil, err
    }

    var job Job
    err = q.db.QueryRowContext(ctx, sql, args...).Scan(
        &job.ID, &job.Workspace, &job.JobType, &job.Payload,
        &job.Priority, &job.Status, &job.Attempts, &job.MaxAttempts,
        &job.ScheduledAt, &job.CreatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, nil // No jobs available
    }
    return &job, err
}

// Complete marks a job as done.
func (q *Queue) Complete(ctx context.Context, jobID int64) error {
    query := qb.Q().Space(`
        UPDATE job_queue SET status = 'done', completed_at = NOW()
        WHERE id = ? AND locked_by = ?`, jobID, q.instanceID)
    sql, args, _ := query.ToSQL()
    _, err := q.db.ExecContext(ctx, sql, args...)
    return err
}

// Fail marks a job as failed. If max attempts exceeded, moves to 'dead'.
func (q *Queue) Fail(ctx context.Context, jobID int64, errMsg string) error {
    query := qb.Q().Space(`
        UPDATE job_queue SET
            status = CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'failed' END,
            locked_by = NULL,
            locked_at = NULL,
            error_msg = ?,
            -- Retry failed jobs with exponential backoff
            scheduled_at = CASE
                WHEN attempts < max_attempts
                THEN NOW() + (interval '1 second' * power(2, attempts))
                ELSE scheduled_at
            END
        WHERE id = ?`, errMsg, jobID)
    sql, args, _ := query.ToSQL()
    _, err := q.db.ExecContext(ctx, sql, args...)
    return err
}

// RecoverStale reclaims jobs locked by crashed workers.
func (q *Queue) RecoverStale(ctx context.Context) (int64, error) {
    query := qb.Q().Space(`
        UPDATE job_queue SET
            status = 'pending',
            locked_by = NULL,
            locked_at = NULL
        WHERE status = 'running'
          AND locked_at < NOW() - interval '?' minute`, int(q.lockTTL.Minutes()))
    sql, args, _ := query.ToSQL()
    result, err := q.db.ExecContext(ctx, sql, args...)
    if err != nil {
        return 0, err
    }
    return result.RowsAffected()
}
```

### 2.3 Worker Pool with Resource Isolation

**File**: `backend/component/jobqueue/worker.go` (new)

```go
package jobqueue

import (
    "context"
    "log/slog"
    "time"

    "github.com/sourcegraph/conc/pool"
)

// WorkerConfig defines resource limits per job type.
type WorkerConfig struct {
    JobTypes       []JobType
    MaxWorkers     int
    PollInterval   time.Duration
}

// Worker consumes jobs from the queue with bounded concurrency.
type Worker struct {
    queue    *Queue
    config   WorkerConfig
    handler  func(ctx context.Context, job *Job) error
}

func NewWorker(queue *Queue, config WorkerConfig,
    handler func(ctx context.Context, job *Job) error) *Worker {
    return &Worker{queue: queue, config: config, handler: handler}
}

// Run starts the worker loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
    p := pool.New().WithMaxGoroutines(w.config.MaxWorkers)

    for {
        select {
        case <-ctx.Done():
            p.Wait()
            return
        default:
        }

        job, err := w.queue.Dequeue(ctx, w.config.JobTypes)
        if err != nil {
            slog.Error("dequeue failed", "error", err)
            time.Sleep(w.config.PollInterval)
            continue
        }
        if job == nil {
            time.Sleep(w.config.PollInterval)
            continue
        }

        p.Go(func() {
            if err := w.handler(ctx, job); err != nil {
                w.queue.Fail(ctx, job.ID, err.Error())
                slog.Error("job failed",
                    "jobID", job.ID, "type", job.JobType, "error", err)
            } else {
                w.queue.Complete(ctx, job.ID)
            }
        })
    }
}
```

### 2.4 Integration with Existing Bus

Giữ Bus channels cho real-time signaling. Thêm job queue cho durable processing.

**File**: `backend/runner/plancheck/scheduler.go` — migration example

```go
// Existing pattern: Bus channel signal → immediate processing
// New pattern: Bus signal → enqueue job → worker processes

func (s *PlanCheckScheduler) Run(ctx context.Context) {
    // Existing: real-time signal from Bus (keep for low-latency)
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-s.bus.PlanCheckTickleChan:
                // Enqueue durable job instead of processing inline
                s.jobQueue.Enqueue(ctx, "system", jobqueue.JobTypePlanCheck,
                    nil, 5, "") // priority 5
            }
        }
    }()

    // New: durable worker consuming from job queue
    worker := jobqueue.NewWorker(s.jobQueue, jobqueue.WorkerConfig{
        JobTypes:     []jobqueue.JobType{jobqueue.JobTypePlanCheck},
        MaxWorkers:   100,
        PollInterval: 1 * time.Second,
    }, s.processPlanCheck)
    worker.Run(ctx)
}
```

### 2.5 NotifyListener Integration

Tận dụng existing PG LISTEN/NOTIFY (TDD §5.4) cho job queue signals.

**File**: `backend/runner/notifylistener/listener.go` — add job_queue channel

```go
// Add to existing LISTEN channels:
_, err = conn.ExecContext(ctx, "LISTEN \"bytebase:job_queue\"")
```

### 2.6 Stale Job Recovery Runner

**File**: `backend/runner/cleaner/cleaner.go` — add to existing DataCleaner

```go
// In existing periodic cleanup loop, add:
func (c *DataCleaner) cleanStaleJobs(ctx context.Context) {
    recovered, err := c.jobQueue.RecoverStale(ctx)
    if err != nil {
        slog.Error("failed to recover stale jobs", "error", err)
        return
    }
    if recovered > 0 {
        slog.Info("recovered stale jobs", "count", recovered)
    }
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L5 (Bus) | **LOW** | Keep channels, add enqueue integration |
| L6 (Runner) | **HIGH** | Runners migrate to consume from job queue |
| L8 (Store) | **MEDIUM** | New job_queue table + queries |
| L10 (Infra) | **LOW** | Migration script, PG LISTEN channel |

---

## 4. Resource Isolation — Worker Config

| Runner | MaxWorkers | Job Types | Priority |
|--------|-----------|-----------|----------|
| TaskRun | 200 | `task_run` | P0 |
| PlanCheck | 100 | `plan_check` | P1 |
| Approval | 50 | `approval` | P1 |
| Cleanup | 10 | `cleanup` | P3 |

---

## 5. Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `JOB_QUEUE_ENABLED` | `true` | Enable PG job queue |
| `JOB_QUEUE_POLL_MS` | `1000` | Polling interval (ms) |
| `JOB_MAX_RETRY` | `3` | Max retry attempts |
| `JOB_LOCK_TTL_MIN` | `5` | Lock timeout (min) |
| `JOB_TASKRUN_WORKERS` | `200` | TaskRun worker pool |
| `JOB_PLANCHECK_WORKERS` | `100` | PlanCheck worker pool |
