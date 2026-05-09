# TASK-WEAK-008-3: Robust Reconnection

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P1 |
| Depends On | TASK-WEAK-008-1 |
| Est. | M (~120 LoC) |

## Objective

Replace `time.Sleep(100ms)` reconnection with file stability check + atomic pool swap + configurable drain timeout. Fixes race condition in `db_connection.go:137-178`.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/pool_manager.go` — add `reloadConnection()` |

## Specification

### File stability check

Read PG URL file twice with 50ms interval, verify stable:
```go
for retries := 0; retries < 5; retries++ {
    url1, _ := readURLFromFile(filePath)
    time.Sleep(50 * time.Millisecond)
    url2, _ := readURLFromFile(filePath)
    if url1 == url2 { newURL = url1; break }
}
```

### Atomic pool swap

```go
pm.mu.Lock()
oldAPI, oldRunner := pm.apiPool, pm.runnerPool
pm.apiPool, pm.runnerPool = newAPI, newRunner
pm.mu.Unlock()
```

### Graceful drain

Old pools drained with configurable timeout (`PG_POOL_DRAIN_TIMEOUT`, default 5min) instead of hardcoded 1 hour.

Metric: `bytebase_db_pool_reconnects_total` counter.

## Acceptance Criteria

- [ ] No `time.Sleep(100ms)` race condition
- [ ] File read stability verified before reconnect
- [ ] Pool swap is atomic (RWMutex)
- [ ] Old pools drained within configurable timeout
- [ ] Reconnect counter incremented
