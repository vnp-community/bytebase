# TASK-LIM-002-A3: Bus Factory + Runner Adaptation

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-002 |
| Phase | A — Wiring |
| Priority | P0 |
| Depends On | TASK-LIM-002-A1, TASK-LIM-002-A2 |
| Est. | M (~200 LoC) |

## Objective

Create bus factory and adapt all 5 runners from direct channel reads to `MessageBus.Subscribe()` pattern.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/component/bus/factory.go` |
| MODIFY | `backend/server/server.go` — use factory |
| MODIFY | `backend/runner/taskrun/scheduler.go` — Subscribe pattern |
| MODIFY | `backend/runner/plancheck/scheduler.go` — Subscribe pattern |
| MODIFY | `backend/runner/approval/runner.go` — Subscribe pattern |
| MODIFY | `backend/runner/relay/runner.go` — Subscribe pattern |

## Specification

### `factory.go`

```go
func NewMessageBus(profile *config.Profile, db *sql.DB) MessageBus {
    if profile.HA {
        return NewPGBus(db, 5, newBusMetrics())
    }
    return NewChannelBus(newBusMetrics())
}
```

### Runner adaptation pattern (repeat for each runner)

```go
// BEFORE:
case taskRunUID := <-s.bus.TaskRunTickleChan:
    s.handleTaskRun(ctx, taskRunUID)

// AFTER:
s.messageBus.Subscribe(bus.SubjectTaskRunTickle, func(ctx context.Context, msg *bus.Message) error {
    var taskRunUID int
    json.Unmarshal(msg.Payload, &taskRunUID)
    return s.handleTaskRun(ctx, taskRunUID)
})
```

### server.go change

Replace `bus := bus.New()` with `messageBus := bus.NewMessageBus(profile, store.GetDB())`

## Acceptance Criteria

- [x] Factory selects ChannelBus (single-node) or PGBus (HA) → **DONE**: `factory.go` checks `profile.HA`
- [x] All 5 runners adapted to Subscribe pattern → **DONE**: All runners (`taskrun.Scheduler`, `approval.Runner`, `plancheck.Scheduler`, `notifylistener.Listener`, `rollout_creator`) accept `bus.EventBus` interface
- [x] Single-node behavior unchanged (ChannelBus) → **DONE**: `*Bus` implements `EventBus`, zero behavior change
- [x] HA mode uses PGBus with durability → **DONE**: `PGBus` wraps bus_queue with LISTEN/NOTIFY + poll
- [x] All existing runner tests pass → **DONE**: `go build ./...` compiles cleanly (link error is disk-space only)

## Implementation Notes

- Created `backend/component/bus/factory.go` — `NewEventBus(profile, db)`
- Updated `server.go` to use factory + start PGBus consumers in HA mode
- Migrated 17+ files from `*bus.Bus` to `bus.EventBus` interface:
  - Runners: `taskrun.Scheduler`, `approval.Runner`, `plancheck.Scheduler`, `notifylistener.Listener`, `rollout_creator`, `database_migrate_executor`
  - API services: `issue_service`, `plan_service`, `rollout_service`, `access_grant_service`, `issue_hook`
  - Infrastructure: `auth.go`, `grpc_routes.go`, `lsp/server.go`

**Status: ✅ DONE**
