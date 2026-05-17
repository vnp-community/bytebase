# TASK-WEAK-008-2: Pool Prometheus Metrics

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P1 |
| Depends On | TASK-WEAK-008-1 |
| Est. | S (~60 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Export per-pool Prometheus metrics every 5s from `sql.DBStats()`.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/pool_manager.go` — add metrics registration |
| EXISTS | `backend/store/pool_metrics.go` — existing metrics collector |

## Implementation Notes

- **Metrics Collection:** Reused the existing `PoolMetrics` implementation in `pool_metrics.go`.
- **Wiring:** Added `StartMetricsCollector` to `PoolManager` which runs the background goroutine to poll `sql.DB.Stats()` every 5 seconds.
- **Labels:** Metrics are exported with a `pool` label (e.g., `api` or `runner`).
- **Additional Metrics:** Added a `reconnects_total` Prometheus counter to track how often the database connection pool is re-initialized (which supports TASK-008-3).

## Acceptance Criteria

- [x] 4 Prometheus gauges exported with `pool` label
- [x] Updated every 5 seconds
- [x] Goroutine stops on context cancellation
