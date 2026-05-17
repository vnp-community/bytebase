-- Trigger 1: Sync workspace + engine when instance changes
CREATE OR REPLACE FUNCTION sync_db_workspace()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.workspace IS DISTINCT FROM NEW.workspace THEN
        UPDATE db SET workspace = NEW.workspace
        WHERE instance = NEW.resource_id;
    END IF;
    IF OLD.metadata->>'engine' IS DISTINCT FROM NEW.metadata->>'engine' THEN
        UPDATE db SET engine = NEW.metadata->>'engine'
        WHERE instance = NEW.resource_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_instance_workspace_sync
    AFTER UPDATE ON instance
    FOR EACH ROW EXECUTE FUNCTION sync_db_workspace();

-- Trigger 2: Auto-compute effective_environment on db insert/update
CREATE OR REPLACE FUNCTION sync_effective_environment()
RETURNS TRIGGER AS $$
BEGIN
    NEW.effective_environment := COALESCE(
        NEW.environment,
        (SELECT environment FROM instance WHERE resource_id = NEW.instance)
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_db_effective_env
    BEFORE INSERT OR UPDATE ON db
    FOR EACH ROW EXECUTE FUNCTION sync_effective_environment();

-- Trigger 3: Propagate instance env change to db
CREATE OR REPLACE FUNCTION sync_instance_env_to_db()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.environment IS DISTINCT FROM NEW.environment THEN
        UPDATE db SET effective_environment = COALESCE(db.environment, NEW.environment)
        WHERE instance = NEW.resource_id AND db.environment IS NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_instance_env_sync
    AFTER UPDATE ON instance
    FOR EACH ROW EXECUTE FUNCTION sync_instance_env_to_db();
