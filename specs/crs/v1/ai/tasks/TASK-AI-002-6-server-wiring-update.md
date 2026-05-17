# TASK-AI-002-6: Server Wiring Update + Compile-Time Checks

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P0 |
| Depends On | TASK-AI-002-2 |
| Status | ✅ DONE |
| Completed | 2025-05-10 |
| Verified | 2025-05-10 |
| Est. | S (update constructor calls + add interface assertions) |

## Delivered

### Changes

| File | Description |
|------|-------------|
| `backend/server/grpc_routes.go` | Added 7 compile-time interface assertions: DataStore, UserStore, SettingReader, WorkspaceReader, AuthStore, AccountReader, PolicyReader |
| `backend/server/grpc_routes.go` | `configureGrpcRouters()` bus parameter: `*bus.Bus` → `bus.EventBus` (user-driven) |

### Compile-Time Checks

```go
var (
    _ store.DataStore       = (*store.Store)(nil)
    _ store.UserStore       = (*store.Store)(nil)
    _ store.SettingReader   = (*store.Store)(nil)
    _ store.WorkspaceReader = (*store.Store)(nil)
    _ store.AuthStore       = (*store.Store)(nil)
    _ store.AccountReader   = (*store.Store)(nil)
    _ store.PolicyReader    = (*store.Store)(nil)
)
```

### EventBus Migration (user-driven, verified here)

The following files were migrated from `*bus.Bus` to `bus.EventBus` interface:

| File | Scope |
|------|-------|
| `api/v1/issue_service.go` | struct + constructor |
| `api/v1/plan_service.go` | struct + constructor |
| `api/v1/rollout_service.go` | struct + constructor |
| `api/v1/issue_hook.go` | function parameter |
| `runner/taskrun/scheduler.go` | struct + constructor |
| `runner/taskrun/rollout_creator.go` | struct + constructor |
| `runner/approval/runner.go` | struct + constructor |
| `runner/plancheck/scheduler.go` | struct + constructor |
| `runner/notifylistener/listener.go` | struct + constructor |
| `server/grpc_routes.go` | function parameter |

**Result**: Zero remaining `*bus.Bus` references in production code.

## Verification (2025-05-10 re-verified)

```bash
go build ./backend/server/...     # ✅ PASS
go build ./backend/api/v1/...     # ✅ PASS
go build ./backend/runner/...     # ✅ PASS
go vet ./backend/server/...       # ✅ PASS
go vet ./backend/api/v1/...       # ✅ PASS
go vet ./backend/runner/...       # ✅ PASS
```

## Acceptance Criteria

- [x] Compile-time interface assertions for 7 store interfaces
- [x] `*bus.Bus` → `bus.EventBus` migration complete (0 remaining refs)
- [x] `go build` passes across all packages
- [x] `go vet` passes
