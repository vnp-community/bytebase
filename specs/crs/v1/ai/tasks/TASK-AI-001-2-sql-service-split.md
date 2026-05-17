# TASK-AI-001-2: sql_service.go Split (→ 7 files)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-001 |
| Priority | P0 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2025-05-09 |
| Verified | 2025-05-10 |
| Est. | M (move ~1800 LoC across files) |

## Objective

Split `sql_service.go` (1877 lines) into 7 domain files. Zero functional change — same package, same struct, method redistribution only.

## Files Created/Modified

| Action | Path | Lines |
|--------|------|-------|
| MODIFY | `backend/api/v1/sql_service.go` — struct + constructor + getUser | 53 |
| CREATE | `backend/api/v1/sql_service_admin.go` — AdminExecute bidi-stream | 115 |
| CREATE | `backend/api/v1/sql_service_query.go` — Query, queryRetry, queryRetryStopOnError, executeWithTimeout | 679 |
| CREATE | `backend/api/v1/sql_service_access.go` — accessCheck, preCheckAccess, prepareRelatedMessage, hasDatabaseAccessRights | 313 |
| CREATE | `backend/api/v1/sql_service_export_ops.go` — Export, doExportFromIssue, doExport, zip/encrypt helpers | 429 |
| CREATE | `backend/api/v1/sql_service_history.go` — SearchQueryHistories, createQueryHistory, Build*MetadataFunc, DiffMetadata, resolveDataSourceID, checkAndGetDataSourceQueriable | 343 |
| EXISTS | `backend/api/v1/sql_service_ai.go` — AI-related query functions (pre-existing) | 380 |

### Verification (2025-05-10 re-verified)

```bash
go build ./backend/api/v1/  # ✅ PASS
go vet ./backend/api/v1/    # ✅ PASS (exit 0)
# Total: 2312 lines across 7 files
```

## Acceptance Criteria

- [x] `sql_service.go` reduced to ≤250 lines (53 lines)
- [x] Each new file ≤700 lines (max: 679 in query.go)
- [x] `go build` passes — no compile errors
- [x] `go vet` passes — no issues
- [x] AdminExecute bidi-stream handling intact
