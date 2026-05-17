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

-- Step 5: Create optimized indexes
CREATE INDEX IF NOT EXISTS idx_db_workspace_not_deleted
    ON db (workspace) WHERE deleted = false;

CREATE INDEX IF NOT EXISTS idx_db_workspace_project_instance_name
    ON db (workspace, project, instance, name) WHERE deleted = false;

CREATE INDEX IF NOT EXISTS idx_db_instance_name
    ON db (instance, name);

CREATE INDEX IF NOT EXISTS idx_db_engine
    ON db (engine) WHERE deleted = false;
