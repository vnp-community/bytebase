# T-007-02: Pool Prometheus Metrics

| Field | Value |
|---|---|
| **Task ID** | T-007-02 |
| **Solution** | SOL-ARCH-007 |
| **Priority** | P0 |
| **Depends On** | T-007-01 |
| **Target File** | `backend/store/pool_metrics.go` |
| **Type** | New file |

---

## Objective

Prometheus metrics per pool: active connections, idle connections, wait duration. Enables Grafana dashboards for pool utilization monitoring.

## Implementation

```go
package store

type poolMetrics struct {
    activeConns  *prometheus.GaugeVec   // labels: pool=api|runner
    idleConns    *prometheus.GaugeVec
    maxConns     *prometheus.GaugeVec
    waitDuration *prometheus.GaugeVec
}

// Collect every 10s from sql.DB.Stats()
func (pm *PoolManager) collectMetrics(ctx context.Context)
```

**Metric names**: `bytebase_db_pool_active_conns`, `bytebase_db_pool_idle_conns`, `bytebase_db_pool_max_conns`, `bytebase_db_pool_wait_duration_seconds`

## Acceptance Criteria

- [ ] 4 Prometheus metrics with `pool` label
- [ ] Collection goroutine runs every 10s
- [ ] `go build ./backend/store/...` passes
