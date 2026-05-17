# T-005: Store — Update CreateDatabaseDefault

| Field | Value |
|-------|-------|
| **Task ID** | T-005 |
| **Solution** | SOL-PERF-001 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | T-001 |
| **Blocks** | None |
| **Status** | DONE |

## Objective

Sửa `CreateDatabaseDefault` để INSERT `workspace` + `engine` khi tạo database mới.

## Target File

`backend/store/database.go` — lines 260-338 (CreateDatabaseDefault)

## Changes

```go
func (s *Store) CreateDatabaseDefault(ctx context.Context, create *DatabaseMessage) (*DatabaseMessage, error) {
    // NEW: Lookup instance to get workspace + engine
    instance, err := s.GetInstanceByResourceID(ctx, create.InstanceID)
    if err != nil || instance == nil {
        return nil, errors.Errorf("instance %q not found", create.InstanceID)
    }

    // MODIFIED: Add workspace + engine to INSERT
    q := qb.Q().Space(`
        INSERT INTO db (instance, project, name, deleted, workspace, engine)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT (instance, name) DO UPDATE SET deleted = EXCLUDED.deleted`,
        create.InstanceID, create.ProjectID, create.DatabaseName, false,
        instance.Workspace, instance.Metadata.GetEngine().String(),
    )
    // ... rest unchanged
}
```

## Verification

- Create a new database → verify `workspace` and `engine` columns populated
- Existing UpsertDatabase functions also need similar update
