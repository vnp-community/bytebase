# T-014: Store — View-Based ListDatabases

| Field | Value |
|-------|-------|
| **Task ID** | T-014 |
| **Solution** | SOL-PERF-005 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | T-013 |
| **Blocks** | None |

## Objective

Sửa `ListDatabases` để skip metadata JSONB deserialization khi `View == BASIC`.

## Target File

`backend/store/database.go` — ListDatabases

## Changes

### Add to FindDatabaseMessage struct:

```go
type DatabaseView int
const (
    DatabaseViewFull  DatabaseView = 0
    DatabaseViewBasic DatabaseView = 1
)

type FindDatabaseMessage struct {
    // ... existing fields ...
    View DatabaseView
}
```

### Modify SELECT column list:

```go
selectCols := qb.Q().Space(`
    db.instance, db.name, db.project, db.workspace,
    db.environment, db.effective_environment, db.engine, db.deleted`)

if find.View == DatabaseViewFull {
    selectCols.Space(", db.metadata, db.db_schema_metadata")
}
```

### Modify row scanning:

```go
if find.View == DatabaseViewBasic {
    err = rows.Scan(&d.InstanceID, &d.DatabaseName, &d.ProjectID, &d.Workspace,
        &d.EnvironmentID, &d.EffectiveEnvironmentID, &d.Engine, &d.Deleted)
} else {
    var metadata, schemaMetadata []byte
    err = rows.Scan(&d.InstanceID, &d.DatabaseName, &d.ProjectID, &d.Workspace,
        &d.EnvironmentID, &d.EffectiveEnvironmentID, &d.Engine, &d.Deleted,
        &metadata, &schemaMetadata)
    // ... unmarshal ...
}
```

## Verification

- BASIC view: no protojson.Unmarshal calls → 4x faster for 1K results
- FULL view: backward compatible, no behavior change
