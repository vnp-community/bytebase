# T-001-03: Enhanced Heartbeat Runner

| Field | Value |
|---|---|
| **Task ID** | T-001-03 |
| **Solution** | SOL-AVAIL-001 |
| **Depends On** | T-001-02 |
| **Target File** | `backend/runner/heartbeat/runner.go` (Modify) |

---

## Objective

Mở rộng heartbeat runner: khởi tạo `ReplicaNode` metadata, gửi DRAINING trước khi shutdown.

## Context — Current code (59 lines total)

```go
type Runner struct {
    store   *store.Store
    profile *config.Profile
}
```

## Implementation

1. Add `node *model.ReplicaNode` field
2. In `NewRunner()` — initialize node with profile metadata
3. In `Run()`:
   - Set status "READY" on start
   - Call `MarkStaleReplicas` on startup
   - On `ctx.Done()` → set "STOPPED" → send final heartbeat with `context.Background()`
4. Add `SetStatus(s)` and `SendHeartbeat()` exported methods for server shutdown use

## Acceptance Criteria

- [x] Runner sends full `ReplicaNode` metadata (version, capabilities, endpoint)
- [x] Marks STOPPED on graceful shutdown
- [x] `SetStatus()` and `SendHeartbeat()` exported for `server.go` to call during shutdown
- [x] `go build ./backend/runner/heartbeat/...` passes
