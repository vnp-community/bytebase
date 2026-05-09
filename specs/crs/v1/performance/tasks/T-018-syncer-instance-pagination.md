# T-018: Syncer — Instance-Based Pagination

| Field | Value |
|-------|-------|
| **Task ID** | T-018 |
| **Solution** | SOL-PERF-003 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | T-004 (workspace filter in ListDatabases) |
| **Blocks** | None |

## Objective

Refactor `trySyncAll` — iterate per-instance thay vì load 200K databases vào memory.

## Target File

`backend/runner/schemasync/syncer.go` — lines 195-215

## Changes

```go
// REPLACE trySyncAll body:
func (s *Syncer) trySyncAll(ctx context.Context) {
    // Load instances (< 10K, manageable)
    instances, err := s.store.ListAllInstances(ctx, false)
    if err != nil {
        slog.Error("failed to list instances", "error", err)
        return
    }

    // Advisory lock (existing infra)
    lock, acquired, err := store.TryAdvisoryLock(ctx, s.store.GetDB(),
        store.AdvisoryLockKeySchemaSyncer)
    if err != nil || !acquired {
        return
    }
    defer lock.Release()

    // Process per-instance (bounded memory)
    pool := s.newAdaptivePool()
    for _, instance := range instances {
        inst := instance
        pool.Go(func() {
            s.syncInstanceDatabases(ctx, inst)
        })
    }
    pool.Wait()
}

func (s *Syncer) syncInstanceDatabases(ctx context.Context, inst *store.InstanceMessage) {
    databases, err := s.store.ListDatabases(ctx, &store.FindDatabaseMessage{
        InstanceID: &inst.ResourceID,
        Workspace:  inst.Workspace,
    })
    if err != nil {
        slog.Error("sync list failed", "instance", inst.ResourceID, "error", err)
        return
    }
    for _, db := range databases {
        s.SyncDatabaseSchema(ctx, db)
    }
}
```

## Memory Impact

- Before: ~2GB (200K × 10KB)
- After: ~2MB per instance batch → ~120MB peak (300 concurrent)
