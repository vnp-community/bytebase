# T-002-02: Leader Election Runner

| Field | Value |
|---|---|
| **Task ID** | T-002-02 |
| **Solution** | SOL-AVAIL-002 |
| **Depends On** | T-002-01, T-001-02 |
| **Target File** | `backend/runner/leader/runner.go` (NEW) |

---

## Objective

Implement leader election runner via PG advisory lock. Leader performs: stale cleanup, orphaned task recovery, cluster health monitoring.

## Implementation

Tạo `backend/runner/leader/runner.go` — xem SOL-AVAIL-002 §2.1.

Key design:
```go
type Runner struct {
    store    *store.Store
    profile  *config.Profile
    isLeader bool
    lock     *store.AdvisoryLock
    mu       sync.RWMutex
}
```

- `Run(ctx, wg)`: 5s ticker, try acquire lock
- `tryAcquireLeadership(ctx)`: `TryAdvisoryLock(key=2001)`, set `isLeader=true`
- `performLeaderDuties(ctx)`: cleanup stale replicas, check cluster health
- `releaseLeadership()`: release lock on shutdown
- `IsLeader() bool`: exported for other runners to check

Wire in `server.go Run()` (HA mode only):
```go
if s.profile.HA {
    s.runnerWG.Add(1)
    go s.leaderRunner.Run(ctx, &s.runnerWG)
}
```

## Acceptance Criteria

- [x] Advisory lock 2001 for leader election
- [x] `IsLeader()` exported method
- [x] Auto-release on ctx cancellation
- [x] Only 1 leader at a time (verified by PG advisory lock)
- [x] `go build ./backend/runner/leader/...` passes
