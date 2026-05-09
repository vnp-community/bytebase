# T-004: Store — Remove JOIN in ListDatabases

| Field | Value |
|-------|-------|
| **Task ID** | T-004 |
| **Solution** | SOL-PERF-001 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | T-001, T-002 |
| **Blocks** | T-018 |

## Objective

Sửa `ListDatabases` trong `database.go` để dùng denormalized columns thay vì JOIN `instance`.

## Target File

`backend/store/database.go` — lines 124-257 (ListDatabases function)

## Changes

### Change 1: Remove unconditional JOIN (around line 134)

```go
// BEFORE:
from.Space("LEFT JOIN instance ON db.instance = instance.resource_id")

// AFTER: Remove this line entirely. Only add JOIN when needed for metadata.
```

### Change 2: Workspace filter uses db.workspace (around line 140)

```go
// BEFORE:
if v := find.Workspace; v != "" {
    where.And("instance.workspace = ?", v)
}

// AFTER:
if v := find.Workspace; v != "" {
    where.And("db.workspace = ?", v)
}
```

### Change 3: Engine filter uses db.engine (around line 160)

```go
// BEFORE:
if v := find.Engine; v != nil {
    where.And("instance.metadata->>'engine' = ?", v.String())
}

// AFTER:
if v := find.Engine; v != nil {
    where.And("db.engine = ?", v.String())
}
```

### Change 4: Environment filter uses effective_environment (around line 146)

```go
// BEFORE:
if v := find.EffectiveEnvironmentID; v != nil {
    where.And(`COALESCE(db.environment, instance.environment) = ?`, *v)
}

// AFTER:
if v := find.EffectiveEnvironmentID; v != nil {
    where.And("db.effective_environment = ?", *v)
}
```

### Change 5: Add workspace to SELECT columns

```go
// Add db.workspace, db.engine, db.effective_environment to SELECT
```

## Verification

- All existing tests in `database_test.go` must pass
- `EXPLAIN ANALYZE` shows no Seq Scan on instance table
