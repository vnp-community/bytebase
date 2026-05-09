# TASK-LIM-001-B2: Server HA Wiring

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | B — Server Integration |
| Priority | P0 |
| Depends On | TASK-LIM-001-B1 |
| Est. | S (~80 LoC) |

## Objective

Wire `LeaderRunner` into `server.go` so that in HA mode, exclusive runners use leader election while shared runners run on all replicas.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/server/server.go` — `Run()` method |

## Specification

In `Run()`, add HA-mode branch:

```go
if s.profile.HA {
    // Exclusive runners — only leader executes
    s.startLeaderRunner(ctx, s.taskScheduler, leader.LockIDTaskScheduler, "TaskScheduler")
    s.startLeaderRunner(ctx, s.schemaSyncer, leader.LockIDSchemaSync, "SchemaSync")
    s.startLeaderRunner(ctx, s.approvalRunner, leader.LockIDApproval, "Approval")
    s.startLeaderRunner(ctx, s.planCheckScheduler, leader.LockIDPlanCheck, "PlanCheck")
    // Shared runners — all replicas
    go s.heartbeatRunner.Run(ctx, &s.runnerWG)
    go s.notifyListener.Run(ctx, &s.runnerWG)
} else {
    // Single-node: existing behavior (unchanged)
}
```

Add helper:
```go
func (s *Server) startLeaderRunner(ctx context.Context, r Runner, lockID int64, name string) {
    elector := leader.NewLeaderElector(s.store.GetDB(), lockID, 10*time.Second)
    wrapped := runner.NewLeaderRunner(r, elector, name)
    s.runnerWG.Add(1)
    go wrapped.Run(ctx, &s.runnerWG)
}
```

## Acceptance Criteria

- [ ] Single-node mode: no behavior change (all runners start directly)
- [ ] HA mode: 4 runners wrapped with LeaderRunner, 2 runners shared
- [ ] Helper method `startLeaderRunner` encapsulates wiring
- [ ] Existing tests pass unchanged
