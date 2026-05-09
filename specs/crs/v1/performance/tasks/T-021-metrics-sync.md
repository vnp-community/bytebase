# T-021: Metrics — Sync Metrics

| Field | Value |
|-------|-------|
| **Task ID** | T-021 |
| **Solution** | SOL-PERF-003 |
| **Type** | New file |
| **Priority** | P2 |
| **Depends on** | None |
| **Blocks** | None |

## Target File

`backend/runner/schemasync/metrics.go` (new)

## Implementation

```go
package schemasync

import "github.com/prometheus/client_golang/prometheus"

var (
    syncCycleDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "bytebase_schema_sync_cycle_duration_seconds",
        Help:    "Duration of a full schema sync cycle",
        Buckets: prometheus.ExponentialBuckets(1, 2, 12),
    })
    syncDatabasesProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_schema_sync_databases_total",
        Help: "Total databases processed",
    }, []string{"status"})
    syncInstanceDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "bytebase_schema_sync_instance_duration_seconds",
        Help:    "Duration per instance",
        Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
    }, []string{"workspace"})
    syncConcurrency = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_schema_sync_concurrency",
        Help: "Current concurrent sync workers",
    })
)

func init() {
    prometheus.MustRegister(syncCycleDuration, syncDatabasesProcessed,
        syncInstanceDuration, syncConcurrency)
}
```
