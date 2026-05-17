# T-001: Migration — Workspace Denormalization

| Field | Value |
|-------|-------|
| **Task ID** | T-001 |
| **Solution** | SOL-PERF-001 |
| **Type** | New file (SQL migration) |
| **Priority** | P0 — Foundation |
| **Depends on** | None |
| **Blocks** | T-004, T-005 |
| **Status** | DONE |

## Objective

Thêm cột `workspace` và `engine` vào bảng `db`, backfill từ bảng `instance`.

## Target File

`backend/migrator/migration/<next_version>/0001_db_workspace_denorm.sql` (new)

## Implementation

```sql
-- Step 1: Add workspace column (nullable first for safe rollout)
ALTER TABLE db ADD COLUMN IF NOT EXISTS workspace TEXT;

-- Step 2: Backfill from instance table
UPDATE db SET workspace = instance.workspace
FROM instance WHERE instance.resource_id = db.instance
AND db.workspace IS NULL;

-- Step 3: Set NOT NULL constraint
ALTER TABLE db ALTER COLUMN workspace SET NOT NULL;
ALTER TABLE db ALTER COLUMN workspace SET DEFAULT '';

-- Step 4: Add engine column
ALTER TABLE db ADD COLUMN IF NOT EXISTS engine TEXT;

UPDATE db SET engine = instance.metadata->>'engine'
FROM instance WHERE instance.resource_id = db.instance
AND db.engine IS NULL;

-- Step 5: Create optimized indexes (CONCURRENTLY = zero downtime)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_workspace_not_deleted
    ON db (workspace) WHERE deleted = false;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_workspace_project_instance_name
    ON db (workspace, project, instance, name) WHERE deleted = false;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_instance_name
    ON db (instance, name);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_db_engine
    ON db (engine) WHERE deleted = false;
```

## Verification

```sql
-- After migration:
SELECT COUNT(*) FROM db WHERE workspace IS NULL;  -- Must be 0
SELECT COUNT(*) FROM db WHERE engine IS NULL;      -- Must be 0

-- Verify index usage:
EXPLAIN ANALYZE SELECT * FROM db WHERE workspace = 'test' AND deleted = false LIMIT 10;
-- Expected: Index Scan using idx_db_workspace_not_deleted
```

## Context Files (read only if needed)

- `backend/store/database.go` — current schema usage
- `backend/migrator/migration/` — migration directory structure
