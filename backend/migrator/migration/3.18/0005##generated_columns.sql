-- Migration: Add generated column for high-frequency JSONB path extraction.
--
-- Extracts issue.payload->>'type' into a STORED generated column with a B-tree index
-- for O(1) filtering. This avoids full table scans when filtering issues by type.
--
-- Naming convention: generated columns use snake_case, JSONB keys use camelCase
-- (from protojson.Marshal).

ALTER TABLE issue ADD COLUMN IF NOT EXISTS issue_type TEXT
    GENERATED ALWAYS AS (payload->>'type') STORED;

CREATE INDEX IF NOT EXISTS idx_issue_type ON issue (issue_type) WHERE issue_type IS NOT NULL;
