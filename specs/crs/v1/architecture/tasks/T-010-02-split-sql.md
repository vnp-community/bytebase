# T-010-02: Split sql_service.go

| Field | Value |
|---|---|
| **Task ID** | T-010-02 |
| **Solution** | SOL-ARCH-010 |
| **Priority** | P2 |
| **Depends On** | None |
| **Target Files** | `backend/api/v1/sql_service.go` → split into 4 files |
| **Type** | Refactor |

---

## Objective

Split `sql_service.go` (1,876 lines) into domain-focused files.

## Implementation

| New File | Domain | Est. Lines |
|----------|--------|------------|
| `sql_service.go` | Struct + constructor | ~100 |
| `sql_service_query.go` | Execute, Query, ExportCSV | ~500 |
| `sql_service_check.go` | SQL Review, syntax check | ~450 |
| `sql_service_ai.go` | AI completion, chat | ~400 |
| `sql_service_export.go` | Data export, streaming | ~426 |

## Acceptance Criteria

- [ ] Each file < 800 lines
- [ ] `go build ./backend/api/v1/...` passes
- [ ] All existing tests pass
