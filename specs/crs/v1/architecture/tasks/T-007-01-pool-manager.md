# T-007-01: Pool Manager Implementation

| Field | Value |
|---|---|
| **Task ID** | T-007-01 |
| **Solution** | SOL-ARCH-007 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/store/pool_manager.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Implement `PoolManager` managing dual `*sql.DB` pools: API pool (70% connections) and Runner pool (30%). Replaces single pool to prevent runner workloads from starving API requests.

## Implementation — DELIVERED

### File: `backend/store/pool_manager.go` (117 lines)

### Types

```go
type PoolType int
const (
    PoolAPI    PoolType = iota  // latency-sensitive
    PoolRunner                   // throughput-oriented
)

type PoolConfig struct {
    PGURL           string
    MaxConns        int      // default: 50
    APIRatio        float64  // default: 0.7
    RunnerRatio     float64  // default: 0.3
    MinConnsPerPool int      // default: 5
}

type PoolManager struct {
    apiPool    *sql.DB
    runnerPool *sql.DB
    config     PoolConfig
}
```

### Key Functions

| Function | Description |
|----------|-------------|
| `NewPoolManager(ctx, cfg)` | Creates dual pools with ratio-based connection allocation |
| `APIPool() *sql.DB` | Returns API connection pool |
| `RunnerPool() *sql.DB` | Returns Runner connection pool |
| `Close() error` | Closes both pools (aggregates errors) |
| `applyPoolDefaults(cfg)` | Sets defaults (50 max, 0.7/0.3 ratio, 5 min per pool) |

### Connection Allocation Logic

```
Total MaxConns = 50 (default)
├─ API Pool:    max(5, floor(50 * 0.7)) = 35 connections
└─ Runner Pool: max(5, floor(50 * 0.3)) = 15 connections
```

- `MinConnsPerPool` guarantees minimum 5 per pool even with extreme ratios
- Each pool gets its own `*sql.DB` with independent `SetMaxOpenConns()`

## Acceptance Criteria

- [x] `PoolManager` creates two separate `*sql.DB` connections ✅
- [x] API pool gets 70% of max connections ✅
- [x] Runner pool gets 30% of max connections ✅
- [x] Minimum 5 connections per pool (`MinConnsPerPool`) ✅
- [x] `go build ./backend/store/...` passes ✅

## Verification

```
$ go build ./backend/store/... → ✅ PASS
$ wc -l backend/store/pool_manager.go → 117
$ grep 'APIRatio' backend/store/pool_manager.go → 0.7 default
$ grep 'RunnerRatio' backend/store/pool_manager.go → 0.3 default
```
