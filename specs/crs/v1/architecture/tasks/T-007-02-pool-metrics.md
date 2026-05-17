# T-007-02: Pool Prometheus Metrics

| Field | Value |
|---|---|
| **Task ID** | T-007-02 |
| **Solution** | SOL-ARCH-007 |
| **Priority** | P0 |
| **Depends On** | T-007-01 |
| **Target File** | `backend/store/pool_metrics.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Prometheus metrics per pool: active connections, idle connections, wait duration. Enables Grafana dashboards for pool utilization monitoring.

## Implementation — DELIVERED

### File: `backend/store/pool_metrics.go` (102 lines)

### Metrics Registered

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `bytebase_db_pool_max_open` | GaugeVec | `pool` | Maximum open connections configured |
| `bytebase_db_pool_open` | GaugeVec | `pool` | Current open connections |
| `bytebase_db_pool_in_use` | GaugeVec | `pool` | Active connections executing queries |
| `bytebase_db_pool_idle` | GaugeVec | `pool` | Idle connections available |
| `bytebase_db_pool_wait_count` | CounterVec | `pool` | Total connection waits |
| `bytebase_db_pool_wait_time` | CounterVec | `pool` | Total wait duration |

### Key Functions

| Function | Description |
|----------|-------------|
| `NewPoolMetrics(registerer)` | Creates and registers all 6 metrics |
| `Collect(poolName, db)` | Snapshots `sql.DB.Stats()` into metrics |
| `RunCollector(ctx, interval, pools)` | Background goroutine collecting every `interval` |

### Collection Architecture

```go
func (m *PoolMetrics) RunCollector(ctx context.Context, interval time.Duration, pools map[string]*sql.DB) {
    ticker := time.NewTicker(interval)
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            for name, db := range pools {
                m.Collect(name, db)  // snapshots sql.DB.Stats()
            }
        }
    }
}
```

- Collects from `sql.DB.Stats()` — zero-allocation read
- Labels: `pool=api` or `pool=runner`
- Default interval: 10s (configurable)

## Acceptance Criteria

- [x] 6 Prometheus metrics with `pool` label (exceeds spec's 4) ✅
- [x] Collection goroutine runs on configurable interval ✅
- [x] `go build ./backend/store/...` passes ✅

## Verification

```
$ go build ./backend/store/... → ✅ PASS
$ wc -l backend/store/pool_metrics.go → 102
$ grep -c 'prometheus\.' backend/store/pool_metrics.go → 12 (6 defs + 6 registrations)
```
