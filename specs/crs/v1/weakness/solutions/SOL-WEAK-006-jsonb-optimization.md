# Solution: JSONB Query Optimization — CR-WEAK-006

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-006                                             |
| **CR Reference**   | CR-WEAK-006                                              |
| **Title**          | GIN Indexes + Field Extraction + Naming Docs             |
| **Affected Layers**| L8 (Store), L10 (Migrator)                               |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per TDD.md §4.4: JSONB columns store protobuf JSON (camelCase via `protojson.Marshal`). Affected: `plan.config`, `task.payload`, `task_run.result`, `policy.payload`, `setting.value`, `issue.payload`.

Per architecture.md §9 (L8): Store uses `pgx/v5` driver. JSONB queries currently use sequential scan — no GIN indexes.

---

## 2. Solution Design

### 2.1 GIN Indexes (CONCURRENTLY to avoid locks)

**Migration**: `backend/migrator/migration/prod/NEXT/0002_gin_indexes.sql`

```sql
-- GIN indexes for frequently queried JSONB columns
-- Using jsonb_path_ops for containment queries (@>)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_task_payload_gin
    ON task USING GIN (payload jsonb_path_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_policy_payload_gin
    ON policy USING GIN (payload jsonb_path_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_payload_gin
    ON issue USING GIN (payload jsonb_path_ops);

-- plan.config is large but less frequently filtered
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plan_config_gin
    ON plan USING GIN (config jsonb_path_ops);
```

**Note**: `CREATE INDEX CONCURRENTLY` cannot run inside a transaction. The migrator must handle this — either by running outside transaction or by using separate migration step.

### 2.2 High-Frequency Field Extraction

Analyze store query patterns to identify top JSONB path filters:

```go
// task.go — current pattern:
// SELECT ... FROM task WHERE payload->>'databaseName' = $1
// This benefits from a dedicated column + index

// Migration:
ALTER TABLE issue ADD COLUMN IF NOT EXISTS issue_type TEXT
    GENERATED ALWAYS AS (payload->>'type') STORED;
CREATE INDEX IF NOT EXISTS idx_issue_type ON issue (issue_type) WHERE issue_type IS NOT NULL;
```

**Store layer** — read from generated column when available:

```go
// issue.go — dual read pattern
func (s *Store) listIssues(ctx context.Context, find *FindIssueMessage) ([]*IssueMessage, error) {
    // Use generated column for type filter if available
    if find.Type != nil {
        where = append(where, fmt.Sprintf("issue_type = $%d", len(args)+1))
        args = append(args, *find.Type)
    }
    // ...
}
```

### 2.3 Naming Convention Documentation

**New file**: `docs/dev/jsonb-naming-convention.md`

```markdown
# JSONB Naming Convention

## Key Rule: JSONB content uses camelCase

PostgreSQL column names = snake_case (standard)
JSONB field keys = camelCase (from protojson.Marshal)

## Correct vs Incorrect Queries

✅ config->>'planConfig'    (camelCase — matches protobuf)
❌ config->>'plan_config'   (snake_case — WILL NOT MATCH)

✅ payload->>'taskRun'
❌ payload->>'task_run'

## Reason
protojson.Marshal() uses camelCase field names from .proto definitions.
```

---

## 3. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/migrator/migration/prod/NEXT/0002_...` | NEW | GIN indexes |
| `backend/migrator/migration/prod/NEXT/0003_...` | NEW | Generated columns |
| `docs/dev/jsonb-naming-convention.md` | NEW | Developer docs |

## 4. Risks

- `CREATE INDEX CONCURRENTLY` may take time on large tables
- Generated columns add write overhead (~2-3%)
- GIN index increases storage by ~10-15%

## 5. Rollback

Drop indexes and generated columns. No data loss.
