# T-004-03: Pool Metrics Exporter

| Field | Value |
|---|---|
| **Task ID** | T-004-03 |
| **Solution** | SOL-AVAIL-004 |
| **Depends On** | T-004-01 |
| **Target File** | `backend/store/db_metrics.go` (Modify) |

---

## Objective

Thêm `PoolMetricsCollector` export `sql.DBStats` sang Prometheus gauges. Tích hợp với Prometheus registry đã có.

## Implementation

Thêm vào file `backend/store/db_metrics.go`:

```go
type PoolMetricsCollector struct {
    db           *sql.DB
    openConns    prometheus.Gauge
    inUseConns   prometheus.Gauge
    idleConns    prometheus.Gauge
    maxOpenConns prometheus.Gauge
}

func NewPoolMetricsCollector(db *sql.DB, registry prometheus.Registerer) *PoolMetricsCollector {
    c := &PoolMetricsCollector{
        db: db,
        openConns:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "bytebase_db_pool_open_connections", Help: "Open connections"}),
        inUseConns:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "bytebase_db_pool_in_use_connections", Help: "In-use connections"}),
        idleConns:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "bytebase_db_pool_idle_connections", Help: "Idle connections"}),
        maxOpenConns: prometheus.NewGauge(prometheus.GaugeOpts{Name: "bytebase_db_pool_max_open_connections", Help: "Max open connections"}),
    }
    registry.MustRegister(c.openConns, c.inUseConns, c.idleConns, c.maxOpenConns)
    return c
}

func (c *PoolMetricsCollector) Collect() {
    stats := c.db.Stats()
    c.openConns.Set(float64(stats.OpenConnections))
    c.inUseConns.Set(float64(stats.InUse))
    c.idleConns.Set(float64(stats.Idle))
    c.maxOpenConns.Set(float64(stats.MaxOpenConnections))
}
```

## Acceptance Criteria

- [x] 4 Prometheus gauges registered: `bytebase_db_pool_{open,in_use,idle,max_open}_connections`
- [x] `Collect()` reads from `sql.DB.Stats()`
- [x] `go build ./backend/store/...` passes
