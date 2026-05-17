# TASK-WEAK-008-4: Runner Pool Integration + Store Wiring

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P0 |
| Depends On | TASK-WEAK-008-1 |
| Est. | M (~100 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Wire PoolManager into Store, deprecate DBConnectionManager, and update runners to use isolated runner pool.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/store.go` — use PoolManager, add `GetRunnerDB()` |
| MODIFY | `backend/store/store_options.go` — update reference to PoolManager |
| MODIFY | `backend/store/db_connection.go` — mark deprecated |
| MODIFY | `backend/runner/taskrun/pending_scheduler.go` — use `store.GetRunnerDB()` |
| MODIFY | `backend/runner/schemasync/syncer.go` — use `store.GetRunnerDB()` |
| MODIFY | `backend/runner/cleaner/data_cleaner.go` — use `store.GetRunnerDB()` |

## Implementation Notes

- **Store Initialization:** Replaced `DBConnectionManager` with `PoolManager` in `Store`'s initialization (`New()` function).
- **Interface Extension:** `Store` now exposes `GetDB()` (returns API pool) and `GetRunnerDB()` (returns Runner pool).
- **Options Sync:** Updated `store_options.go` to properly reference `s.poolManager.GetPgURL()`.
- **Runner Migration:** 
  - `pending_scheduler.go` (long migrations) now uses `GetRunnerDB()` for its transactions.
  - `syncer.go` (schema sync) now uses `GetRunnerDB()` for its advisory lock.
  - `data_cleaner.go` (batch deletes) now uses `GetRunnerDB()` for bus queue cleanup.
- **Deprecation:** Added `DEPRECATED` comment to `db_connection.go` to signal its impending removal in v7.0.

## Acceptance Criteria

- [x] `store.GetDB()` returns API pool (backward compat)
- [x] `store.GetRunnerDB()` returns isolated runner pool
- [x] Heavy runners use runner pool
- [x] Existing service code unaffected (uses `GetDB()`)
