# TASK-AI-001-5: Infrastructure Services Split

| Field | Value |
|-------|-------|
| Solution | SOL-AI-001 |
| Priority | P1 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2025-05-10 |
| Verified | 2025-05-10 |
| Est. | M (2 services × 2 files each) |

## Objective

Split `database_service.go` (1247 LoC) and `instance_service.go` (1181 LoC) into 2 files each.

## Delivered

| File | Lines | Content |
|------|-------|---------|
| `database_service.go` | 363 | Struct + CRUD (Get/BatchGet/List/Update/BatchUpdate) |
| `database_service_sync.go` | 901 | SyncDatabase, GetMetadata, GetSchema, DiffSchema, GetSchemaString, SDL |
| `instance_service.go` | 761 | Struct + CRUD (Get/List/Create/Update/Delete/Sync/Batch) |
| `instance_service_activation.go` | 437 | AddDataSource, UpdateDataSource, RemoveDataSource |

### Verification (2025-05-10 re-verified)

```bash
go build ./backend/api/v1/...  # ✅ PASS
go vet ./backend/api/v1/...    # ✅ PASS
# Total: 2462 lines across 4 files
```

## Acceptance Criteria

- [x] Each original file significantly reduced
- [x] `go build` passes
- [x] `go vet` passes
