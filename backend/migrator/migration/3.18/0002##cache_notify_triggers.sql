-- Cache invalidation triggers for cross-replica cache coherence.
-- When a row is inserted, updated, or deleted in key tables, a PG NOTIFY
-- event is sent on the 'cache_invalidation' channel so that all replicas
-- can invalidate their cached copies.

-- Generic notification function that fires on any row change.
CREATE OR REPLACE FUNCTION notify_cache_invalidation() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('cache_invalidation',
        json_build_object(
            'table', TG_TABLE_NAME,
            'action', TG_OP,
            'id', COALESCE(
                CASE WHEN TG_OP = 'DELETE' THEN OLD.resource_id ELSE NEW.resource_id END,
                ''::TEXT
            )
        )::TEXT
    );
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Apply to principal (user cache)
CREATE TRIGGER trg_cache_inv_principal
    AFTER INSERT OR UPDATE OR DELETE ON principal
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();

-- Apply to instance (instance cache)
CREATE TRIGGER trg_cache_inv_instance
    AFTER INSERT OR UPDATE OR DELETE ON instance
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();

-- Apply to db (database cache)
CREATE TRIGGER trg_cache_inv_db
    AFTER INSERT OR UPDATE OR DELETE ON db
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();

-- Apply to project (project cache)
CREATE TRIGGER trg_cache_inv_project
    AFTER INSERT OR UPDATE OR DELETE ON project
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();

-- Apply to policy (policy cache)
CREATE TRIGGER trg_cache_inv_policy
    AFTER INSERT OR UPDATE OR DELETE ON policy
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();

-- Apply to setting (setting cache)
CREATE TRIGGER trg_cache_inv_setting
    AFTER INSERT OR UPDATE OR DELETE ON setting
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();
