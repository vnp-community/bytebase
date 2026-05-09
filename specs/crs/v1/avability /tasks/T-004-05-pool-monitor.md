# T-004-05: Pool Monitor Runner

| Field | Value |
|---|---|
| **Task ID** | T-004-05 |
| **Solution** | SOL-AVAIL-004 |
| **Depends On** | T-004-03 |
| **Target Files** | `backend/runner/monitor/pool_monitor.go` (NEW), `backend/server/server.go` (Modify) |

---

## Objective

Tạo `PoolMonitor` runner (theo pattern `MemoryMonitor` đã có) và wire vào server lifecycle.

## Implementation

### 1. New file: `backend/runner/monitor/pool_monitor.go`

```go
package monitor

type PoolMonitor struct {
    db         *sql.DB
    metricsCol *store.PoolMetricsCollector
}

func NewPoolMonitor(db *sql.DB, metricsCol *store.PoolMetricsCollector) *PoolMonitor {
    return &PoolMonitor{db: db, metricsCol: metricsCol}
}

func (m *PoolMonitor) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            m.metricsCol.Collect()
            stats := m.db.Stats()
            if stats.MaxOpenConnections > 0 {
                util := float64(stats.InUse) / float64(stats.MaxOpenConnections)
                if util > 0.8 {
                    slog.Warn("DB pool high utilization",
                        slog.Float64("utilization", util),
                        slog.Int("inUse", stats.InUse),
                        slog.Int("maxOpen", stats.MaxOpenConnections))
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 2. Wire in `server.go` Run() — after existing MemoryMonitor

```go
// Add after line ~287 (mmm := monitor.NewMemoryMonitor)
s.runnerWG.Add(1)
poolMon := monitor.NewPoolMonitor(stores.GetDB(), poolMetricsCollector)
go poolMon.Run(ctx, &s.runnerWG)
```

## Acceptance Criteria

- [ ] `PoolMonitor` follows same runner pattern as `MemoryMonitor`
- [ ] Logs warning when pool utilization > 80%
- [ ] Wired into `Server.Run()` lifecycle
- [ ] `go build ./backend/...` passes
