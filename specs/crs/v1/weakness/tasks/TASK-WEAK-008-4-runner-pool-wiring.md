# TASK-WEAK-008-4: Runner Pool Integration + Store Wiring

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P0 |
| Depends On | TASK-WEAK-008-1 |
| Est. | M (~100 LoC) |

## Objective

Wire PoolManager into Store, deprecate DBConnectionManager, and update runners to use isolated runner pool.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/store.go` — use PoolManager, add `GetRunnerDB()` |
| MODIFY | `backend/store/db_connection.go` — mark deprecated |
| MODIFY | `backend/runner/taskrun/scheduler.go` — use `store.GetRunnerDB()` |
| MODIFY | `backend/runner/schemasync/syncer.go` — use `store.GetRunnerDB()` |
| MODIFY | `backend/server/server.go` — pass PoolConfig |

## Specification

### `store.go`

```go
type Store struct {
    poolManager *PoolManager  // replaces dbConnManager
}

func (s *Store) GetDB() *sql.DB { return s.poolManager.GetDefaultDB() }     // backward compat
func (s *Store) GetRunnerDB() *sql.DB { return s.poolManager.GetDB(PoolRunner) }  // NEW
```

### Runner integration

Runners that do heavy DB operations use `store.GetRunnerDB()`:
- TaskRun scheduler (long migrations)
- SchemaSync syncer (bulk schema reads)
- DataCleaner (batch deletes)

Other runners continue using `store.GetDB()` (API pool).

### `db_connection.go`

Add `// DEPRECATED: Use pool_manager.go. This file will be removed in v7.0.`

## Acceptance Criteria

- [ ] `store.GetDB()` returns API pool (backward compat)
- [ ] `store.GetRunnerDB()` returns isolated runner pool
- [ ] Heavy runners use runner pool
- [ ] Existing service code unaffected (uses `GetDB()`)
- [ ] All tests pass
