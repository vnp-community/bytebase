ALTER TABLE task_run ADD COLUMN IF NOT EXISTS assigned_node TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_task_run_assigned_node ON task_run (assigned_node) WHERE status = 'RUNNING';
