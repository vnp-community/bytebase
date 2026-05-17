# T-008: Store — Adaptive Cache Sizing

| Field | Value |
|-------|-------|
| **Task ID** | T-008 |
| **Solution** | SOL-PERF-002 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | T-010 |
| **Status** | DONE |

## Objective

Thay hardcoded cache sizes (32768, 128) bằng adaptive sizing dựa trên actual entity count.

## Target File

`backend/store/store.go` — `New()` function

## Changes

### Add helper functions:

```go
func adaptiveCacheSize(entityCount, minSize, maxSize, coveragePct int) int {
    target := entityCount * coveragePct / 100
    if target < minSize { return minSize }
    if target > maxSize { return maxSize }
    return target
}

func getEntityCount(ctx context.Context, db *sql.DB, table, condition string) int {
    var count int
    query := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", table, condition)
    if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
        return 0
    }
    return count
}
```

### Modify cache initialization in `New()`:

```go
// BEFORE:
databaseCache, err := lru.New[string, *DatabaseMessage](32768)
dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](128, nil, 5*time.Minute)

// AFTER:
dbCount := getEntityCount(ctx, connMgr.GetDB(), "db", "deleted = false")
instanceCount := getEntityCount(ctx, connMgr.GetDB(), "instance", "deleted = false")

dbCacheSize := adaptiveCacheSize(dbCount, 32768, 500000, 50)
schemaL1Size := adaptiveCacheSize(dbCount, 512, 5000, 2)
instanceCacheSize := adaptiveCacheSize(instanceCount, 1024, 32768, 80)

databaseCache, err := lru.New[string, *DatabaseMessage](dbCacheSize)
dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](schemaL1Size, nil, 10*time.Minute)
```

## Imports to add

```go
"fmt"
```

## Verification

- With 200K DBs: cache size = 100K (50% coverage)
- With 0 DBs: cache size = 32768 (minimum)
