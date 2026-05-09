# TASK-WEAK-004-2: sql_service.go Split (→ 5 files)

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-004 |
| Priority | P1 |
| Depends On | — |
| Est. | M (move ~1500 LoC across files) |

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

- [ ] `sql_service.go` reduced to ≤400 lines
- [ ] `go build ./backend/api/v1/...` passes
- [ ] All existing integration tests pass
