# T-002: Migration — Effective Environment Materialization

| Field | Value |
|-------|-------|
| **Task ID** | T-002 |
| **Solution** | SOL-PERF-001 |
| **Type** | New file (SQL migration) |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | T-004 |

## Objective

Thêm cột `effective_environment` vào bảng `db`, pre-compute thay vì runtime COALESCE.

## Target File

`backend/migrator/migration/<next_version>/0002_db_effective_env.sql` (new)

## Implementation

```sql
ALTER TABLE db ADD COLUMN IF NOT EXISTS effective_environment TEXT;

UPDATE db SET effective_environment = COALESCE(
    db.environment,
    (SELECT environment FROM instance WHERE resource_id = db.instance)
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_effective_env
    ON db (effective_environment) WHERE deleted = false;
```

## Verification

```sql
SELECT COUNT(*) FROM db WHERE effective_environment IS NULL AND deleted = false;
-- Should be 0

EXPLAIN ANALYZE SELECT * FROM db
WHERE effective_environment = 'prod' AND deleted = false LIMIT 10;
-- Expected: Index Scan using idx_db_effective_env
```
