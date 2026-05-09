# TASK-WEAK-006-1: GIN Index Migration

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-006 |
| Priority | P1 |
| Depends On | — |
| Est. | S (~20 LoC SQL) |

## Objective

Add GIN indexes (`jsonb_path_ops`) on frequently queried JSONB columns. Use `CREATE INDEX CONCURRENTLY` to avoid locking.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/migrator/migration/prod/NEXT/0002_gin_indexes.sql` |

## Specification

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_task_payload_gin
    ON task USING GIN (payload jsonb_path_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_policy_payload_gin
    ON policy USING GIN (payload jsonb_path_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_payload_gin
    ON issue USING GIN (payload jsonb_path_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plan_config_gin
    ON plan USING GIN (config jsonb_path_ops);
```

**Note**: `CONCURRENTLY` cannot run inside a transaction — migrator must handle separately.

## Acceptance Criteria

- [ ] Indexes created without table locks
- [ ] Containment queries (`@>`) use GIN index (EXPLAIN)
- [ ] No performance regression on write operations (< 5% overhead)
