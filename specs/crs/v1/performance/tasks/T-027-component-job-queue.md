# T-027: Component — Job Queue Manager

| Field | Value |
|-------|-------|
| **Task ID** | T-027 |
| **Solution** | SOL-PERF-006 |
| **Type** | New file |
| **Priority** | P0 |
| **Depends on** | T-026 |
| **Blocks** | T-028 |

## Target File

`backend/component/jobqueue/queue.go` (new)

## Key Methods

1. `Enqueue(ctx, workspace, jobType, payload, priority, dedupKey)` — INSERT with ON CONFLICT DO NOTHING
2. `Dequeue(ctx, jobTypes)` — SELECT FOR UPDATE SKIP LOCKED + UPDATE status='running'
3. `Complete(ctx, jobID)` — UPDATE status='done'
4. `Fail(ctx, jobID, errMsg)` — UPDATE status='failed'/'dead' + exponential backoff
5. `RecoverStale(ctx)` — reclaim jobs locked > lockTTL

## Full code

See SOL-PERF-006 §2.2 for complete implementation (~130 lines).

Core dequeue pattern:

```go
func (q *Queue) Dequeue(ctx context.Context, jobTypes []JobType) (*Job, error) {
    // UPDATE job_queue SET status='running', locked_by=?, locked_at=NOW()
    // WHERE id = (
    //   SELECT id FROM job_queue WHERE status='pending' AND scheduled_at <= NOW()
    //   AND job_type IN (?) ORDER BY priority DESC, scheduled_at ASC
    //   LIMIT 1 FOR UPDATE SKIP LOCKED
    // ) RETURNING ...
}
```

## Dependencies

Uses existing `backend/common/qb` query builder pattern.
