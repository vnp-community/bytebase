# T-007-01: Pool Manager Implementation

| Field | Value |
|---|---|
| **Task ID** | T-007-01 |
| **Solution** | SOL-ARCH-007 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/store/pool_manager.go` |
| **Type** | New file |

---

## Objective

Implement `PoolManager` managing dual `*sql.DB` pools: API pool (70% connections) and Runner pool (30%). Replaces single pool to prevent runner workloads from starving API requests.

## Implementation

```go
package store

type PoolType int
const (
    PoolAPI    PoolType = iota
    PoolRunner
)

type PoolManager struct {
    apiPool    *sql.DB
    runnerPool *sql.DB
}

type PoolConfig struct {
    PGURL      string
    MaxConns   int     // total (default: 50)
    APIRatio   float64 // default: 0.7
    RunnerRatio float64 // default: 0.3
}

func NewPoolManager(ctx context.Context, cfg PoolConfig) (*PoolManager, error)
func (pm *PoolManager) APIPool() *sql.DB
func (pm *PoolManager) RunnerPool() *sql.DB
func (pm *PoolManager) Close() error
```

## Acceptance Criteria

- [ ] `PoolManager` creates two separate `*sql.DB` connections
- [ ] API pool gets 70% of max connections
- [ ] Runner pool gets 30% of max connections
- [ ] Minimum 5 connections per pool
- [ ] `go build ./backend/store/...` passes
- [ ] Unit test: verify pool counts
