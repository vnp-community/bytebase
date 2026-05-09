# T-015: Store — Cursor-Based Pagination

| Field | Value |
|-------|-------|
| **Task ID** | T-015 |
| **Solution** | SOL-PERF-005 |
| **Type** | Edit file |
| **Priority** | P1 |
| **Depends on** | T-013 |
| **Blocks** | None |

## Objective

Thêm cursor-based keyset pagination vào `ListDatabases` thay thế OFFSET.

## Target File

`backend/store/database.go` — ListDatabases

## Changes

### Add to FindDatabaseMessage:

```go
AfterCursor *string // Format: "project:instance:name"
```

### Add cursor WHERE clause (before ORDER BY):

```go
if cursor := find.AfterCursor; cursor != nil {
    parts := strings.SplitN(*cursor, ":", 3)
    if len(parts) == 3 {
        where.And(`(db.project, db.instance, db.name) > (?, ?, ?)`,
            parts[0], parts[1], parts[2])
    }
}
```

### Build next_page_cursor from last result:

```go
// After query execution, build cursor from last row
if len(databases) > 0 {
    last := databases[len(databases)-1]
    nextCursor := fmt.Sprintf("%s:%s:%s", last.ProjectID, last.InstanceID, last.DatabaseName)
}
```

## Verification

- OFFSET 50000 on 200K rows: ~200ms → cursor seek: ~15ms
