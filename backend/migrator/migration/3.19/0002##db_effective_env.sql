ALTER TABLE db ADD COLUMN IF NOT EXISTS effective_environment TEXT;

UPDATE db SET effective_environment = COALESCE(
    db.environment,
    (SELECT environment FROM instance WHERE resource_id = db.instance)
);

CREATE INDEX IF NOT EXISTS idx_db_effective_env
    ON db (effective_environment) WHERE deleted = false;
