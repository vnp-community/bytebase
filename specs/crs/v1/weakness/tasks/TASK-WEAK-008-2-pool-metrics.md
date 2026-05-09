# TASK-WEAK-008-2: Pool Prometheus Metrics

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P1 |
| Depends On | TASK-WEAK-008-1 |
| Est. | S (~60 LoC) |

## Objective

Export per-pool Prometheus metrics every 5s from `sql.DBStats()`.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/pool_manager.go` — add `collectMetrics()` |

## Specification

Metrics (labeled by `pool=api|runner`):
- `bytebase_db_pool_active_connections{pool}`
- `bytebase_db_pool_idle_connections{pool}`
- `bytebase_db_pool_waiting_requests{pool}`
- `bytebase_db_pool_max_connections{pool}`

```go
func (pm *PoolManager) collectMetrics(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    for { /* read Stats(), set gauges */ }
}
```

Started automatically in `Initialize()`.

## Acceptance Criteria

- [ ] 4 Prometheus gauges exported with `pool` label
- [ ] Updated every 5 seconds
- [ ] Goroutine stops on context cancellation
