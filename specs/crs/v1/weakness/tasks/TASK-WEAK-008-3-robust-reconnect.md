# TASK-WEAK-008-3: Robust Reconnection

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P1 |
| Depends On | TASK-WEAK-008-1 |
| Est. | M (~120 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Replace `time.Sleep(100ms)` reconnection with file stability check + atomic pool swap + configurable drain timeout. Fixes race condition.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/pool_manager.go` — add `reloadConnection()` |

## Implementation Notes

- **File Stability Check:** Implemented `readStableURL` to read the PG URL file twice with a 50ms delay, verifying the content is identical. This avoids reading a partially written file.
- **Atomic Pool Swap:** Reconnection creates *new* pools completely before swapping. The swap is guarded by a `sync.RWMutex` to ensure zero downtime and no race conditions for in-flight requests calling `GetDB()`.
- **Graceful Drain:** The old `apiPool` and `runnerPool` are passed to an asynchronous `drainPool` function.
- **Drain Config:** Drain time is configurable via `PoolConfig.DrainTimeout` (defaults to 5 minutes instead of the old hardcoded 1 hour). After the timeout, connections are forcefully closed to free resources.
- **Observability:** Reconnections increment the `bytebase_db_pool_reconnects_total` metric.

## Acceptance Criteria

- [x] No `time.Sleep(100ms)` race condition
- [x] File read stability verified before reconnect
- [x] Pool swap is atomic (RWMutex)
- [x] Old pools drained within configurable timeout
- [x] Reconnect counter incremented
