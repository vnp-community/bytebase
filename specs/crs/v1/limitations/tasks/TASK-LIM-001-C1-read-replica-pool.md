# TASK-LIM-001-C1: Read Replica Pool Manager

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | C — Read Replica |
| Priority | P2 |
| Depends On | TASK-LIM-001-A3 |
| Est. | M (~200 LoC) |

## Objective

Create `PoolManager` supporting primary (read-write) and optional read replica (read-only) with lag-aware routing.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/pool.go` |
| CREATE | `backend/store/pool_test.go` |

## Specification

```go
type PoolManager struct {
    primary      *sql.DB
    replica      *sql.DB        // nil if no replica
    replicaLag   atomic.Int64   // microseconds
    lagThreshold time.Duration
}
```

Key methods:
- `NewPoolManager(primaryURL, replicaURL, lagThreshold)` — open connections, start lag monitor
- `ForRead() *sql.DB` — replica if available AND lag < threshold, else primary
- `ForWrite() *sql.DB` — always primary
- `monitorReplicaLag(ctx)` — every 5s: `SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp()))`
- Prometheus: `bytebase_db_replica_lag_seconds`

Config: `PG_READ_REPLICA_URL` env var, `REPLICA_LAG_THRESHOLD` (default 5s)

## Acceptance Criteria

- [ ] No replica URL → `ForRead()` returns primary
- [ ] Replica available + lag OK → `ForRead()` returns replica
- [ ] Replica lag exceeds threshold → `ForRead()` falls back to primary
- [ ] Replica connection failure → graceful fallback + warning log
- [ ] Lag monitoring goroutine with proper context cancellation
