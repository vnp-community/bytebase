# TASK-LIM-001-A4: PG NOTIFY Cache Invalidator

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | A — Invalidation |
| Priority | P1 |
| Depends On | TASK-LIM-001-A3 |
| Est. | S (~120 LoC) |

## Objective

Tạo `CacheInvalidator` runner lắng nghe PG NOTIFY events để xóa cache entries tương ứng. Đảm bảo cache coherence giữa nhiều replicas khi dùng Redis.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/cache_invalidator.go` |
| CREATE | `backend/migrator/migration/<next>/0002_cache_notify_triggers.sql` |

## Specification

### `cache_invalidator.go`

- Struct `CacheInvalidator` with `store *Store`, `pgPool *pgxpool.Pool`
- `Run(ctx, wg)` — runner pattern: `LISTEN cache_invalidation` → `WaitForNotification` loop
- `handleNotification(ctx, payload)` — parse JSON `{table, action, id}` → route to appropriate cache `.Delete()`
- Table → cache mapping: `principal→userEmailCache`, `instance→instanceCache`, `db→databaseCache`, `project→projectCache`, `policy→policyCache`, `setting→settingCache`

### SQL triggers

```sql
CREATE OR REPLACE FUNCTION notify_cache_invalidation() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('cache_invalidation',
        json_build_object('table', TG_TABLE_NAME, 'action', TG_OP, 'id', COALESCE(NEW.resource_id, OLD.resource_id, ''::TEXT))::TEXT
    );
    RETURN COALESCE(NEW, OLD);
END; $$ LANGUAGE plpgsql;

-- Apply to key tables
CREATE TRIGGER trg_cache_inv_principal AFTER INSERT OR UPDATE OR DELETE ON principal FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();
-- Similar for: instance, db, project, policy, setting
```

## Acceptance Criteria

- [ ] Runner starts and listens on `cache_invalidation` channel
- [ ] INSERT/UPDATE/DELETE on key tables triggers cache invalidation
- [ ] Reconnects after PG connection drop
- [ ] Graceful shutdown on context cancellation
