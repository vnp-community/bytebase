# T-007-03: Store + Runner Pool Wiring

| Field | Value |
|---|---|
| **Task ID** | T-007-03 |
| **Solution** | SOL-ARCH-007 |
| **Priority** | P0 |
| **Depends On** | T-007-01 |
| **Target Files** | `backend/store/store.go`, `backend/server/server.go`, `backend/runner/schemasync/syncer.go`, `backend/runner/taskrun/scheduler.go` |
| **Type** | Modify existing |

---

## Objective

Wire PoolManager into Store and route runners to RunnerPool. API services continue using APIPool (default). Feature flag `PG_POOL_ISOLATION` enables dual pool.

## Implementation

### 1. Store — add `GetRunnerDB()`
```go
func (s *Store) GetRunnerDB() *sql.DB {
    if s.poolManager != nil { return s.poolManager.RunnerPool() }
    return s.dbConnManager.GetDB()
}
```

### 2. Server — init PoolManager when flag enabled
```go
if profile.PoolIsolation {
    pm, err := store.NewPoolManager(ctx, store.PoolConfig{...})
    stores.SetPoolManager(pm)
}
```

### 3. Runners — use `GetRunnerDB()` for heavy queries

## Acceptance Criteria

- [ ] `PG_POOL_ISOLATION=false` → single pool (no change)
- [ ] `PG_POOL_ISOLATION=true` → dual pool active
- [ ] Runners use runner pool; API uses API pool
- [ ] `go build ./backend/...` passes
- [ ] Existing tests pass
