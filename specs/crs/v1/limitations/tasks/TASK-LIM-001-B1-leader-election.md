# TASK-LIM-001-B1: Leader Election + Runner Wrapper

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | B — Leader Election |
| Priority | P0 |
| Depends On | — |
| Est. | M (~200 LoC) |

## Objective

Implement leader election using PG advisory locks (zero new dependency). Create `LeaderRunner` wrapper that only executes inner runner when elected leader.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/leader/election.go` |
| CREATE | `backend/component/leader/election_test.go` |
| CREATE | `backend/runner/leader_runner.go` |

## Specification

### `election.go` — LeaderElector

```go
type LeaderElector struct {
    db        *sql.DB
    lockID    int64
    isLeader  atomic.Bool
    renewTick time.Duration
}

const (
    LockIDTaskScheduler int64 = 100001
    LockIDPlanCheck     int64 = 100002
    LockIDSchemaSync    int64 = 100003
    LockIDApproval      int64 = 100004
    LockIDDataCleaner   int64 = 100005
)
```

Key methods:
- `Run(ctx)` — ticker loop: `tryAcquire()` every `renewTick`
- `tryAcquire()` — `SELECT pg_try_advisory_lock($1)` → update `isLeader`
- `release()` — `SELECT pg_advisory_unlock($1)`
- `IsLeader() bool` — atomic read

### `leader_runner.go` — LeaderRunner wrapper

```go
type LeaderRunner struct {
    inner   Runner
    elector *leader.LeaderElector
    name    string
}
```

- `Run(ctx, wg)` — starts elector in goroutine, polls `IsLeader()` every 1s, when true: starts `inner.Run()`
- `Runner` interface: `Run(ctx context.Context, wg *sync.WaitGroup)`

## Acceptance Criteria

- [ ] Advisory lock acquired/released correctly
- [ ] Only 1 replica holds lock per lockID (test with 2 concurrent electors)
- [ ] Session-level lock auto-releases on connection close (crash safety)
- [ ] LeaderRunner starts inner runner only when elected
- [ ] Prometheus metric `bytebase_leader_status` exported
