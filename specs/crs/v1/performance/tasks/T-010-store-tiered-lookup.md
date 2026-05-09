# T-010: Store — Tiered Cache Lookup in GetDBSchema

| Field | Value |
|-------|-------|
| **Task ID** | T-010 |
| **Solution** | SOL-PERF-002 |
| **Type** | Edit file |
| **Priority** | P0 |
| **Depends on** | T-008, T-009 |
| **Blocks** | None |

## Objective

Sửa `GetDBSchema` để dùng L1→L2→DB tiered lookup thay vì chỉ L1.

## Target File

`backend/store/db_schema.go` — GetDBSchema function

## Changes

```go
func (s *Store) GetDBSchema(ctx context.Context, find *FindDBSchemaMessage) (*model.DatabaseMetadata, error) {
    cacheKey := getDBSchemaCacheKey(find.InstanceID, find.DatabaseName)

    // L1: Fast in-memory (hot entries)
    if v, ok := s.dbSchemaCache.Get(cacheKey); ok {
        return v, nil
    }

    // L2: Compressed (larger capacity, slower)
    if s.dbSchemaL2Cache != nil {
        if v, ok := s.dbSchemaL2Cache.Get(cacheKey); ok {
            s.dbSchemaCache.Add(cacheKey, v) // Promote to L1
            return v, nil
        }
    }

    // L3: Database query
    result, err := s.queryDBSchema(ctx, find)
    if err != nil {
        return nil, err
    }
    if result != nil {
        s.dbSchemaCache.Add(cacheKey, result)
        if s.dbSchemaL2Cache != nil {
            s.dbSchemaL2Cache.Add(cacheKey, result)
        }
    }
    return result, nil
}
```

## Note

- Extract existing DB query logic into `queryDBSchema()` private method
- Also update any `UpdateDBSchema` / `UpsertDBSchema` to write to both L1 and L2
