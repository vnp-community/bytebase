# Solution: Automated Failover & Disaster Recovery

| Field          | Value                                    |
|----------------|------------------------------------------|
| **Solution ID**| SOL-AVAIL-002                            |
| **CR ID**      | CR-AVAIL-002                             |
| **Status**     | Draft                                    |
| **Created**    | 2026-05-08                               |
| **Layers**     | L5 (Component), L6 (Runner), L8 (Store)  |

---

## 1. Analysis — Existing Infrastructure

### 1.1 Điểm tận dụng

| Component | File | Capability |
|---|---|---|
| **Advisory Locks** | `backend/store/advisory_lock.go` | `TryAdvisoryLock(key)` — 3 lock keys defined (1001-1003), session-level |
| **Bus channels** | `backend/component/bus/bus.go` | 5 buffered channels (1000 cap each), `sync.Map` for cancel funcs |
| **TaskRun scheduler** | `backend/runner/taskrun/scheduler.go` | Executor registration, PENDING→RUNNING transition |
| **PG NOTIFY** | `backend/runner/notifylistener/` | `bytebase:plan_check`, `bytebase:task_run` channels |
| **Heartbeat** | `backend/store/replica_heartbeat.go` | `CountActiveReplicas(within)`, `DeleteStaleReplicaHeartbeats(olderThan)` |
| **Shutdown** | `backend/server/server.go:313` | `cancel()` → `runnerWG.Wait()` — runners check `ctx.Done()` |

### 1.2 Key Gap Analysis

```
Current state:
- Advisory locks cho PendingScheduler, Migration, SchemaSyncer
- Nếu node holding lock dies → lock auto-released (PG session end)
- NHƯNG: chưa có mechanism detect lock-holder death và re-acquire
- Bus messages mất khi crash (Go channels, no persistence)
- TaskRuns RUNNING khi node crash → stuck forever (no recovery)
```

---

## 2. Giải pháp kỹ thuật

### 2.1 Leader Election via Advisory Lock Extension

**Strategy**: Mở rộng advisory lock pattern đã có, thêm `AdvisoryLockKeyLeader` cho leader election. Leader chịu trách nhiệm failover monitoring.

```go
// backend/store/advisory_lock.go — Add new lock keys
const (
    AdvisoryLockKeyPendingScheduler AdvisoryLockKey = 1001
    AdvisoryLockKeyMigration        AdvisoryLockKey = 1002
    AdvisoryLockKeySchemaSyncer     AdvisoryLockKey = 1003
    // NEW:
    AdvisoryLockKeyLeader           AdvisoryLockKey = 2001  // Cluster leader
    AdvisoryLockKeyBackup           AdvisoryLockKey = 2002  // Backup coordinator
    AdvisoryLockKeyHealthMonitor    AdvisoryLockKey = 2003  // Health monitor
)
```

**New file**: `backend/runner/leader/runner.go`

```go
package leader

import (
    "context"
    "log/slog"
    "sync"
    "time"
    
    "github.com/bytebase/bytebase/backend/store"
)

const (
    leaderCheckInterval = 5 * time.Second
    staleNodeThreshold  = 30 * time.Second
)

// Runner attempts to acquire leader lock and performs leader-only duties.
type Runner struct {
    store   *store.Store
    profile *config.Profile
    
    isLeader bool
    lock     *store.AdvisoryLock
    mu       sync.RWMutex
    
    // Leader-only responsibilities
    failoverMonitor *FailoverMonitor
    taskRecoverer   *TaskRecoverer
}

func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(leaderCheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            r.tryAcquireLeadership(ctx)
            if r.IsLeader() {
                r.performLeaderDuties(ctx)
            }
        case <-ctx.Done():
            r.releaseLeadership()
            return
        }
    }
}

func (r *Runner) tryAcquireLeadership(ctx context.Context) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if r.lock != nil {
        return // Already leader
    }
    
    lock, acquired, err := store.TryAdvisoryLock(ctx, r.store.GetDB(), store.AdvisoryLockKeyLeader)
    if err != nil {
        slog.Error("Failed to try leader lock", slog.String("error", err.Error()))
        return
    }
    if acquired {
        r.lock = lock
        r.isLeader = true
        slog.Info("Acquired leader role", slog.String("replica", r.profile.ReplicaID))
    }
}

func (r *Runner) performLeaderDuties(ctx context.Context) {
    // 1. Cleanup stale replicas
    staleCount, _ := r.store.DeleteStaleReplicaHeartbeats(ctx, staleNodeThreshold * 3)
    if staleCount > 0 {
        slog.Warn("Cleaned stale replicas", slog.Int64("count", staleCount))
    }
    
    // 2. Recover orphaned task runs (from crashed nodes)
    r.recoverOrphanedTaskRuns(ctx)
    
    // 3. Monitor cluster health
    r.checkClusterHealth(ctx)
}

func (r *Runner) releaseLeadership() {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.lock != nil {
        r.lock.Release()
        r.lock = nil
        r.isLeader = false
    }
}
```

### 2.2 Orphaned Task Recovery

**Problem**: Khi node chạy `TaskRun` bị crash, task stays RUNNING forever.

**Solution**: Leader periodically scans for orphaned tasks (RUNNING status on dead nodes).

```go
// backend/runner/leader/task_recovery.go
package leader

type TaskRecoverer struct {
    store *store.Store
    bus   *bus.Bus
}

func (tr *TaskRecoverer) RecoverOrphanedTasks(ctx context.Context) error {
    // 1. Get list of active replicas
    activeReplicas, err := tr.store.ListActiveReplicas(ctx, 30*time.Second)
    if err != nil {
        return err
    }
    activeSet := make(map[string]bool)
    for _, r := range activeReplicas {
        activeSet[r.ReplicaID] = true
    }
    
    // 2. Find RUNNING task_runs assigned to dead nodes
    // task_run table doesn't have node_id yet → need migration
    orphanedRuns, err := tr.store.FindOrphanedTaskRuns(ctx, activeSet)
    if err != nil {
        return err
    }
    
    for _, run := range orphanedRuns {
        slog.Warn("Recovering orphaned task run",
            slog.Int64("taskRunID", run.ID),
            slog.String("deadNode", run.AssignedNode))
        
        // 3. Reset to PENDING for re-execution
        if err := tr.store.ResetTaskRunToPending(ctx, run.ID, "recovered from crashed node"); err != nil {
            slog.Error("Failed to reset task run", slog.Int64("id", run.ID), slog.String("error", err.Error()))
            continue
        }
        
        // 4. Tickle the bus to pick up the reset task
        tr.bus.TaskRunTickleChan <- int(run.TaskID)
    }
    
    return nil
}
```

**Database migration** — Add `assigned_node` to task_run:

```sql
-- Track which node is executing each task_run
ALTER TABLE task_run
    ADD COLUMN IF NOT EXISTS assigned_node TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_task_run_assigned_node
    ON task_run (assigned_node) WHERE status = 'RUNNING';
```

**Modify TaskRun scheduler** — Record assignment:

```go
// backend/runner/taskrun/scheduler.go — When starting a task
func (s *Scheduler) startTask(ctx context.Context, task *store.TaskMessage, taskRunUID int64) {
    // Record which node is executing this
    s.store.UpdateTaskRunAssignedNode(ctx, taskRunUID, s.profile.ReplicaID)
    
    // ... existing execution logic
}
```

### 2.3 Bus Message Durability (PG-backed)

**Problem**: `bus.Bus` uses Go channels → messages lost on crash. TDD §5.1 confirms this trade-off.

**Solution**: Dual-write pattern — write to PG table + push to channel. Leader reads from PG on recovery.

```go
// backend/component/bus/persistent.go (NEW)
package bus

import "database/sql"

// PersistentBus wraps Bus with PG-backed durability for critical messages.
type PersistentBus struct {
    *Bus
    db *sql.DB
}

func (pb *PersistentBus) PublishTaskRunTickle(ctx context.Context, taskID int) error {
    // 1. Write to PG (durable)
    _, err := pb.db.ExecContext(ctx, `
        INSERT INTO bus_message (channel, payload, created_at, processed)
        VALUES ('task_run_tickle', $1::TEXT, now(), false)
    `, taskID)
    if err != nil {
        return err
    }
    
    // 2. Push to channel (in-memory fast path)
    select {
    case pb.TaskRunTickleChan <- taskID:
    default:
        // Channel full, PG has the record — leader will recover
    }
    
    // 3. PG NOTIFY for cross-instance
    _, _ = pb.db.ExecContext(ctx, "SELECT pg_notify('bytebase:task_run', $1::TEXT)", taskID)
    
    return nil
}
```

**Recovery on startup** (leader-only):

```go
func (pb *PersistentBus) RecoverUnprocessedMessages(ctx context.Context) error {
    rows, err := pb.db.QueryContext(ctx, `
        SELECT id, channel, payload FROM bus_message
        WHERE processed = false
        AND created_at > now() - INTERVAL '1 hour'
        ORDER BY created_at
    `)
    // ... dispatch each message to appropriate channel
    // ... mark as processed
}
```

### 2.4 DR Procedure Runner

**New file**: `backend/runner/dr/runner.go`

```go
package dr

// DRRunner manages disaster recovery drills and failover procedures.
type DRRunner struct {
    store      *store.Store
    profile    *config.Profile
    bus        *bus.Bus
    isLeader   func() bool
}

// ExecuteFailover orchestrates the failover procedure.
func (r *DRRunner) ExecuteFailover(ctx context.Context, params FailoverParams) (*FailoverResult, error) {
    result := &FailoverResult{StartedAt: time.Now()}
    
    // Step 1: Verify failover is needed (multi-check)
    if !params.Force {
        if active, _ := r.store.CountActiveReplicas(ctx, 30*time.Second); active > 0 {
            return nil, errors.New("active replicas exist, failover not needed")
        }
    }
    
    // Step 2: Acquire failover lock (prevent concurrent failovers)
    lock, acquired, err := store.TryAdvisoryLock(ctx, r.store.GetDB(), store.AdvisoryLockKeyLeader)
    if !acquired {
        return nil, errors.New("another failover in progress")
    }
    defer lock.Release()
    
    // Step 3: Log failover event
    event := &store.FailoverEvent{
        EventType:   "APPLICATION_FAILOVER",
        Trigger:     params.Trigger,
        InitiatedBy: params.InitiatedBy,
    }
    r.store.CreateFailoverEvent(ctx, event)
    
    // Step 4: Recover orphaned tasks
    // ... (see TaskRecoverer above)
    
    // Step 5: Verify service health
    // ...
    
    result.CompletedAt = time.Now()
    result.Duration = result.CompletedAt.Sub(result.StartedAt)
    return result, nil
}
```

### 2.5 Failover Event Logging

```sql
-- Failover audit trail
CREATE TABLE IF NOT EXISTS failover_event (
    id              BIGSERIAL   PRIMARY KEY,
    event_type      TEXT        NOT NULL,
    trigger         TEXT        NOT NULL DEFAULT 'AUTOMATIC',
    source_node     TEXT,
    initiated_by    TEXT        NOT NULL DEFAULT 'SYSTEM',
    status          TEXT        NOT NULL DEFAULT 'IN_PROGRESS',
    details         JSONB       NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    duration_ms     FLOAT,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 3. Integration with Server Lifecycle

**File**: `backend/server/server.go` — Add leader runner to the run loop.

```go
func (s *Server) Run(ctx context.Context, port int) error {
    ctx, cancel := context.WithCancel(ctx)
    s.cancel = cancel
    
    // Existing runners...
    s.runnerWG.Add(1)
    go s.taskScheduler.Run(ctx, &s.runnerWG)
    // ...
    
    // NEW: Leader election runner (only in HA mode)
    if s.profile.HA {
        s.runnerWG.Add(1)
        go s.leaderRunner.Run(ctx, &s.runnerWG)
    }
    
    // ... rest of existing code
}
```

---

## 4. File Change Summary

| Layer | File | Change Type | Description |
|---|---|---|---|
| L8 | `backend/store/advisory_lock.go` | **Modify** | Add lock keys 2001-2003 |
| L6 | `backend/runner/leader/runner.go` | **New** | Leader election runner |
| L6 | `backend/runner/leader/task_recovery.go` | **New** | Orphaned task recovery |
| L6 | `backend/runner/dr/runner.go` | **New** | DR procedure runner |
| L5 | `backend/component/bus/persistent.go` | **New** | PG-backed message persistence |
| L8 | `backend/store/failover_event.go` | **New** | Failover event CRUD |
| L8 | `backend/store/task_run.go` | **Modify** | Add `assigned_node`, `FindOrphanedTaskRuns` |
| L2 | `backend/server/server.go` | **Modify** | Add leader runner to lifecycle |
| L10 | `backend/migrator/migration/X.Y.Z/` | **New** | task_run.assigned_node, failover_event, bus_message tables |

---

## 5. Backward Compatibility

| Scenario | Behavior |
|---|---|
| Single-node | Leader runner acquires lock immediately, no-op leader duties |
| HA without leader runner | Existing advisory lock behavior unchanged, no recovery |
| Migration rollback | `assigned_node` DEFAULT '' — no impact on existing queries |
| Bus messages | Existing channel-based path preserved as fast path, PG is additional durability layer |

---

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Leader election thrashing | LOW | 5s check interval, advisory lock is session-level (stable) |
| Orphaned task false positive | MEDIUM | Only recover tasks on nodes absent from heartbeat > 30s |
| Bus PG writes add latency | LOW | Async write, channel is primary path. PG only for durability |
| Concurrent failover | HIGH | Advisory lock prevents concurrent execution |
