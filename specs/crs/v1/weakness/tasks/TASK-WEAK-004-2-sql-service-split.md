# TASK-WEAK-004-2: sql_service.go Split (→ 5 files)

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P1 |
| Depends On | — |
| Est. | M (move ~1500 LoC across files) |
| Status | ✅ Done |
| Completed | 2026-05-12 |

## Objective

Split `sql_service.go` (1876 lines) into 5 domain files. Same package, same struct.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/sql_service.go` — keep struct + constructor |
| CREATE | `backend/api/v1/sql_query.go` — Execute, Query, AdminExecute streaming |
| CREATE | `backend/api/v1/sql_check.go` — Check (SQL review), advisor integration |
| CREATE | `backend/api/v1/sql_ai.go` — NL2SQL, AI explanation, suggestion |
| CREATE | `backend/api/v1/sql_export.go` — Export (CSV, JSON, XLSX), format |

## Acceptance Criteria

- [x] `sql_service.go` reduced to 53 lines (≤400 ✓)
- [x] `go build ./backend/api/v1/...` passes
- [x] All existing integration tests pass

## Implementation Notes

- Split into 8 files: `sql_service.go` (53), `sql_service_query.go` (679), `sql_service_export_ops.go` (429), `sql_service_ai.go` (380), `sql_service_history.go` (343), `sql_service_access.go` (313), `sql_service_admin.go` (115), `sql_service_converter.go` (68)
- All files ≤800 lines, well within 1500-line lint threshold
