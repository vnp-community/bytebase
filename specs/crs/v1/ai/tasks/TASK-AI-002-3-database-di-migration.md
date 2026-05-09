# TASK-AI-002-3: DatabaseService DI Migration

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P1 |
| Depends On | TASK-AI-002-1 |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Est. | M |

## Objective

Replace `*store.Store` in `DatabaseService` with `store.DataStore` aggregate interface.

## Changes

| File | Change |
|------|--------|
| `backend/store/interfaces.go` | Added `DatabaseWriter` interface (+`UpdateDatabase`) |
| `backend/store/interfaces.go` | Added `SyncHistoryReader` interface (+`GetSyncHistory`) |
| `backend/store/interfaces.go` | Added `GetEnvironmentByID` to `SettingReader` |
| `backend/store/interfaces.go` | Expanded `DataStore` with `DatabaseWriter` + `SyncHistoryReader` |
| `backend/api/v1/database_service.go` | `store *store.Store` → `store store.DataStore` |
| `backend/api/v1/database_service.go` | `NewDatabaseService(store *store.Store, ...)` → `NewDatabaseService(store store.DataStore, ...)` |

### Why This Works Without Cascading

DatabaseService methods call `s.store.GetDatabase()`, `s.store.UpdateDatabase()`, etc. — all now covered by `DataStore`. No helper functions outside the service take the store parameter from `DatabaseService`, so no cascade.

## Verification

```bash
go build ./backend/store/...   # ✅ PASS
go build ./backend/api/v1/...  # ✅ PASS
go build ./backend/server/...  # ✅ PASS
go vet ./backend/api/v1/...    # ✅ PASS
```

## Acceptance Criteria

- [x] `DatabaseService.store` type is `store.DataStore` (interface)
- [x] `go build` passes for api/v1, store, and server packages
- [x] `go vet` passes
- [x] Zero breaking changes to callers (grpc_routes.go passes *Store which satisfies DataStore)
