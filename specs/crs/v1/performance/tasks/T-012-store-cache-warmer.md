# T-012: Store — Cache Warmer

| Field | Value |
|-------|-------|
| **Task ID** | T-012 |
| **Solution** | SOL-PERF-002 |
| **Type** | New file |
| **Priority** | P2 |
| **Depends on** | None |
| **Blocks** | None |

## Target File

`backend/store/cache_warmer.go` (new)

## Implementation

```go
package store

import (
    "context"
    "log/slog"
    "time"
)

func (s *Store) WarmDatabaseCache(ctx context.Context) {
    if !s.enableCache {
        return
    }
    start := time.Now()
    limit := 10000

    databases, err := s.ListDatabases(ctx, &FindDatabaseMessage{
        Limit: &limit,
    })
    if err != nil {
        slog.Warn("Cache warming failed", "error", err)
        return
    }

    slog.Info("Cache warming completed",
        "databases", len(databases),
        "duration", time.Since(start),
    )
}
```

## Integration

Call `store.WarmDatabaseCache(ctx)` after store initialization in server startup.
