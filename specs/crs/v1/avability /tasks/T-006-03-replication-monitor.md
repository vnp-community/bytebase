# T-006-03: Replication Lag Monitor

| Field | Value |
|---|---|
| **Task ID** | T-006-03 |
| **Solution** | SOL-AVAIL-006 |
| **Depends On** | T-006-01 |
| **Target Files** | `backend/runner/replication/monitor.go` (NEW), `backend/server/server.go` (Modify) |

---

## Objective

Monitor PG streaming replication lag. Primary checks `pg_stat_replication`, standby checks `pg_last_xact_replay_timestamp`.

## Implementation

Xem SOL-AVAIL-006 §2.4. Key design:

```go
type Monitor struct {
    db          *sql.DB
    profile     *config.Profile
    lagGauge    prometheus.Gauge   // bytebase_replication_lag_seconds
    statusGauge prometheus.Gauge   // bytebase_replication_status (0/1/2)
}
```

- `Run(ctx, wg)`: branch on `IsPrimary()` vs standby
- **Primary monitor**: query `pg_stat_replication` → log each standby's lag
- **Standby monitor**: `EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))`
  - < 30s → SYNCED (2)
  - 30-300s → LAGGING (1)
  - > 300s → BROKEN (0)

### Wire in server.go (multi-region mode):

```go
if s.profile.RegionRole != "" {
    s.runnerWG.Add(1)
    go s.replicationMonitor.Run(ctx, &s.runnerWG)
}
```

## Acceptance Criteria

- [ ] 2 Prometheus gauges: lag_seconds, replication_status
- [ ] Primary queries `pg_stat_replication`
- [ ] Standby queries replay timestamp
- [ ] CRITICAL log when lag > 300s
- [ ] 30s check interval
- [ ] `go build ./backend/runner/replication/...` passes
