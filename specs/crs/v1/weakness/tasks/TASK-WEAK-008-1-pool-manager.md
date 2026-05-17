# TASK-WEAK-008-1: PoolManager + Dual Pool Architecture

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-008 |
| Priority | P0 |
| Depends On | — |
| Est. | L (~300 LoC) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Replace `DBConnectionManager` with `PoolManager` providing dual connection pools: API pool (70%) and Runner pool (30%). Removes 50-connection hard cap.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/pool_manager.go` |
| EXITS | `backend/store/pool_metrics.go` |

## Implementation Notes

- **Dual Pools:** Implemented `PoolManager` that maintains two isolated `*sql.DB` instances (`apiPool` and `runnerPool`).
- **Auto-Detection:** Added `probeMaxConnections` to query PG for `max_connections` and `superuser_reserved_connections`. Uses 70% of available connections to leave room for other clients. Default limits applied.
- **Configurable Ratios:** Pool sizing respects the `APIRatio` (70%) and `RunnerRatio` (30%) configuration with minimum 5 connections per pool.
- **Backwards Compatibility:** Added `GetDefaultDB()` returning `PoolAPI` to preserve compatibility with existing `Store.GetDB()` callers.
- **Thread Safety:** Added `sync.RWMutex` to protect pool pointers during reconnection swaps.

## Acceptance Criteria

- [x] Two separate `*sql.DB` pools created
- [x] Auto-detect PG max_connections when not configured
- [x] `GetDefaultDB()` returns API pool (backward compat)
- [x] `GetDB(PoolRunner)` returns isolated runner pool
- [x] Pool sizes respect min/max bounds
