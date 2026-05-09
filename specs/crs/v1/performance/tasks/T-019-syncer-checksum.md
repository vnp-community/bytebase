# T-019: Syncer — Checksum-Based Skip

| Field | Value |
|-------|-------|
| **Task ID** | T-019 |
| **Solution** | SOL-PERF-003 |
| **Type** | New file |
| **Priority** | P1 |
| **Depends on** | None |
| **Blocks** | None |

## Objective

Tạo `checksum.go` — so sánh schema hash trước khi full sync, skip nếu unchanged.

## Target File

`backend/runner/schemasync/checksum.go` (new)

## Implementation

```go
package schemasync

import (
    "context"
    "time"
    "github.com/bytebase/bytebase/backend/store"
)

const forceFullSyncInterval = 24 * time.Hour

func (s *Syncer) shouldSkipSync(ctx context.Context, db *store.DatabaseMessage) bool {
    if db.Metadata == nil {
        return false
    }
    lastSync := db.Metadata.GetLastSyncTime()
    if lastSync.IsValid() && time.Since(lastSync.AsTime()) > forceFullSyncInterval {
        return false
    }
    remoteChecksum, err := s.getRemoteSchemaChecksum(ctx, db)
    if err != nil {
        return false
    }
    return remoteChecksum == db.Metadata.GetSchemaChecksum()
}

func (s *Syncer) getRemoteSchemaChecksum(ctx context.Context, db *store.DatabaseMessage) (string, error) {
    driver, err := s.dbFactory.GetAdminDatabaseDriver(ctx, db.InstanceID, db)
    if err != nil {
        return "", err
    }
    defer driver.Close(ctx)

    rows, err := driver.QueryConn(ctx, nil, `
        SELECT md5(string_agg(
            t.table_name || ':' || t.column_count::text,
            '|' ORDER BY t.table_name
        ))
        FROM (
            SELECT c.table_name, COUNT(cols.column_name) as column_count
            FROM information_schema.tables c
            LEFT JOIN information_schema.columns cols
                ON cols.table_schema = c.table_schema AND cols.table_name = c.table_name
            WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
              AND c.table_type = 'BASE TABLE'
            GROUP BY c.table_name
        ) t`, nil)
    if err != nil {
        return "", err
    }
    if len(rows) > 0 && len(rows[0].Rows) > 0 {
        return rows[0].Rows[0].Values[0].GetStringValue(), nil
    }
    return "", nil
}
```

## Note

Expected ~70% skip rate (most schemas don't change between 15min intervals).
