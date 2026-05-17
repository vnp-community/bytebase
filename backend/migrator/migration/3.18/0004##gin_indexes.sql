-- Migration: Add GIN indexes (jsonb_path_ops) on frequently queried JSONB columns.

CREATE INDEX IF NOT EXISTS idx_task_payload_gin
    ON task USING GIN (payload jsonb_path_ops);
CREATE INDEX IF NOT EXISTS idx_policy_payload_gin
    ON policy USING GIN (payload jsonb_path_ops);
CREATE INDEX IF NOT EXISTS idx_issue_payload_gin
    ON issue USING GIN (payload jsonb_path_ops);
CREATE INDEX IF NOT EXISTS idx_plan_config_gin
    ON plan USING GIN (config jsonb_path_ops);
