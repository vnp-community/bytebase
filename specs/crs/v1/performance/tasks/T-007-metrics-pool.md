# T-007: Metrics — Pool Metrics

| Field | Value |
|-------|-------|
| **Task ID** | T-007 |
| **Solution** | SOL-PERF-001 |
| **Type** | New file |
| **Priority** | P2 |
| **Depends on** | None |
| **Blocks** | None |
| **Status** | DONE |

## Objective

Expose connection pool stats qua Prometheus gauges.

## Target File

`backend/store/db_metrics.go` (new)

## Implementation

```go
package store

import "github.com/prometheus/client_golang/prometheus"

func (s *Store) RegisterPoolMetrics() {
    prometheus.MustRegister(
        prometheus.NewGaugeFunc(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_open_connections",
            Help: "Number of open connections to metadata DB",
        }, func() float64 { return float64(s.GetDB().Stats().OpenConnections) }),

        prometheus.NewGaugeFunc(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_in_use",
            Help: "Number of connections currently in use",
        }, func() float64 { return float64(s.GetDB().Stats().InUse) }),

        prometheus.NewGaugeFunc(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_idle",
            Help: "Number of idle connections",
        }, func() float64 { return float64(s.GetDB().Stats().Idle) }),

        prometheus.NewGaugeFunc(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_wait_count",
            Help: "Total number of connections waited for",
        }, func() float64 { return float64(s.GetDB().Stats().WaitCount) }),
    )
}
```

## Integration Point

Call `s.store.RegisterPoolMetrics()` in server startup (e.g., `backend/server/server.go`).
