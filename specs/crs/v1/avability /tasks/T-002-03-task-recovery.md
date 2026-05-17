# T-002-03: Orphaned Task Recovery

| Field | Value |
|---|---|
| **Task ID** | T-002-03 |
| **Solution** | SOL-AVAIL-002 |
| **Depends On** | T-002-02 |
| **Target Files** | `backend/runner/leader/task_recovery.go` (NEW), migration SQL |

---

## Objective

Detect RUNNING task_runs on dead nodes → reset to PENDING for re-execution.

## Implementation

### 1. Migration: add `assigned_node` to `task_run`

```sql
ALTER TABLE task_run ADD COLUMN IF NOT EXISTS assigned_node TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_task_run_assigned_node ON task_run (assigned_node) WHERE status = 'RUNNING';
```

### 2. Store method: `FindOrphanedTaskRuns(ctx, activeNodeSet)`

Query: SELECT task_runs WHERE status='RUNNING' AND assigned_node NOT IN (active set).

### 3. `TaskRecoverer` — xem SOL-AVAIL-002 §2.2

- Compare RUNNING tasks against active replicas
- Reset orphans to PENDING with audit note
- Tickle bus channel for re-pickup

### 4. Modify `taskrun/scheduler.go` — `startTask()`

Add: `s.store.UpdateTaskRunAssignedNode(ctx, taskRunUID, s.profile.ReplicaID)`

## Acceptance Criteria

- [x] Migration adds `assigned_node` column
- [x] `FindOrphanedTaskRuns` finds RUNNING tasks on dead nodes
- [x] Reset to PENDING with recovery note
- [x] Task scheduler records `assigned_node` on start
- [x] `go build ./backend/...` passes
