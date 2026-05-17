# T-001-04: Enhanced Graceful Shutdown

| Field | Value |
|---|---|
| **Task ID** | T-001-04 |
| **Solution** | SOL-AVAIL-001 |
| **Depends On** | T-001-03 |
| **Target File** | `backend/server/server.go` (Modify) |

---

## Objective

Mở rộng `Shutdown()` (line 313-353): tăng timeout 10s→30s, mark DRAINING, ordered cleanup.

## Context — Current Shutdown (line 313)

```go
func (s *Server) Shutdown(ctx context.Context) error {
    slog.Info("Stopping Bytebase...")
    ctx, cancel := context.WithTimeout(ctx, gracefulShutdownPeriod) // 10s
    defer cancel()
    if s.cancel != nil { s.cancel() }
    if s.httpServer != nil { s.httpServer.Shutdown(ctx) }
    s.runnerWG.Wait()
    if s.store != nil { s.store.Close() }
    // ...
}
```

## Implementation

1. Change `gracefulShutdownPeriod` from 10s → 30s (line 57)
2. Add DRAINING step BEFORE http shutdown:
   ```go
   if s.heartbeatRunner != nil {
       s.heartbeatRunner.SetStatus("DRAINING")
       s.heartbeatRunner.SendHeartbeat(context.Background())
   }
   ```
3. Add STOPPED step AFTER runners wait:
   ```go
   if s.heartbeatRunner != nil {
       s.heartbeatRunner.SetStatus("STOPPED")
       s.heartbeatRunner.SendHeartbeat(context.Background())
   }
   ```
4. Reorder: DRAINING → HTTP shutdown → cancel runners → wait → STOPPED → close DB

## Acceptance Criteria

- [x] `gracefulShutdownPeriod` = 30s
- [x] DRAINING heartbeat sent before HTTP shutdown
- [x] STOPPED heartbeat sent after runners exit
- [x] Existing cleanup order preserved (sample instances, PG stoppers)
- [x] `go build ./backend/server/...` passes
