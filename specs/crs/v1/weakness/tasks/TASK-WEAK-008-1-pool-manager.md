# TASK-WEAK-008-1: PoolManager + Dual Pool Architecture

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P0 |
| Depends On | — |
| Est. | L (~300 LoC) |

## Objective

Replace `DBConnectionManager` with `PoolManager` providing dual connection pools: API pool (70%) and Runner pool (30%). Removes 50-connection hard cap.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/pool_manager.go` |
| CREATE | `backend/store/pool_manager_test.go` |

## Specification

### `pool_manager.go`

```go
type PoolConfig struct {
    MaxConnections int     // PG_MAX_CONNECTIONS, 0 = auto-detect
    APIPoolRatio   float64 // PG_API_POOL_RATIO, default 0.7
    DrainTimeout   time.Duration
}

type PoolManager struct {
    apiPool    *sql.DB
    runnerPool *sql.DB
    config     PoolConfig
    metrics    *poolMetrics
}
```

Key methods:
- `Initialize(ctx)` — probe PG `max_connections`, create dual pools
- `GetDB(PoolType) *sql.DB` — select API or Runner pool
- `GetDefaultDB() *sql.DB` — API pool (backward compat for `store.GetDB()`)
- `Close()` — close both pools

Pool sizing:
- Auto-detect: `70% * (pg_max_connections - reserved_connections)`
- API pool: `effective * ratio` (default 70%)
- Runner pool: `effective - apiConns`
- Minimum: API=5, Runner=5, Total=10; Maximum: 200

Config: `PG_MAX_CONNECTIONS`, `PG_API_POOL_RATIO`, `PG_POOL_DRAIN_TIMEOUT`

## Acceptance Criteria

- [ ] Two separate `*sql.DB` pools created
- [ ] Auto-detect PG max_connections when not configured
- [ ] `GetDefaultDB()` returns API pool (backward compat)
- [ ] `GetDB(PoolRunner)` returns isolated runner pool
- [ ] Pool sizes respect min/max bounds
- [ ] Unit test: verify dual pool creation and sizing
