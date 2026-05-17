# TASK-WEAK-006-2: Generated Column + Store Dual Read

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-006 |
| Priority | P2 |
| Depends On | TASK-WEAK-006-1 |
| Est. | M (~100 LoC) |
| Status | ✅ Done |

## Objective

Extract high-frequency JSONB paths into generated columns with B-tree indexes for O(1) filtering.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/migrator/migration/prod/NEXT/0003_generated_columns.sql` |
| MODIFY | `backend/store/issue.go` — use generated column for type filter |
| CREATE | `docs/dev/jsonb-naming-convention.md` — developer documentation |

## Specification

### Migration

```sql
ALTER TABLE issue ADD COLUMN IF NOT EXISTS issue_type TEXT
    GENERATED ALWAYS AS (payload->>'type') STORED;
CREATE INDEX IF NOT EXISTS idx_issue_type ON issue (issue_type) WHERE issue_type IS NOT NULL;
```

### Store dual read

```go
if find.Type != nil {
    where = append(where, fmt.Sprintf("issue_type = $%d", len(args)+1))
    args = append(args, *find.Type)
}
```

### JSONB naming docs

Document: column names = snake_case, JSONB keys = camelCase (from `protojson.Marshal`).

## Acceptance Criteria

- [x] Generated column populated correctly from JSONB
- [x] B-tree index used for type filter (EXPLAIN)
- [x] Store uses generated column when filter present
- [x] Naming convention documented

## Implementation Notes

Created files:
- `backend/migrator/migration/3.18/0005_generated_columns.sql` — migration
- `docs/dev/jsonb-naming-convention.md` — developer documentation

Modified:
- `backend/store/issue.go` — ListIssues now queries `issue.issue_type` instead
  of `issue.type` when filtering by Types, leveraging the B-tree index on the
  generated column.
- `backend/migrator/migration/LATEST.sql` — schema updated with generated column
  and index.
