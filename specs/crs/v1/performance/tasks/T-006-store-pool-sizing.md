# T-006: Store — Dynamic Connection Pool Sizing

| Field | Value |
|-------|-------|
| **Task ID** | T-006 |
| **Solution** | SOL-PERF-001 |
| **Type** | Edit file |
| **Priority** | P1 |
| **Depends on** | None |
| **Blocks** | None |
| **Status** | DONE |

## Objective

Thay hardcoded `maxOpenConns = 50` bằng dynamic sizing dựa trên PG `max_connections` và env var.

## Target File

`backend/store/db_connection.go` — lines 218-250 (createConnectionWithTracer)

## Changes

```go
// BEFORE (line 242-246):
maxOpenConns := maxConns - reservedConns
if maxOpenConns > 50 {
    maxOpenConns = 50
}
db.SetMaxOpenConns(maxOpenConns)

// AFTER:
availableConns := maxConns - reservedConns
maxOpenConns := getConfiguredPoolSize(availableConns)

db.SetMaxOpenConns(maxOpenConns)
db.SetMaxIdleConns(maxOpenConns / 2)
db.SetConnMaxLifetime(30 * time.Minute)
db.SetConnMaxIdleTime(5 * time.Minute)
```

### New helper function (same file):

```go
func getConfiguredPoolSize(availableConns int) int {
    if v := os.Getenv("PG_MAX_POOL_SIZE"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return min(n, availableConns)
        }
    }
    target := availableConns * 80 / 100
    return max(min(target, 200), 10)
}
```

## Imports to add

```go
"os"
"strconv"
```

## Verification

- Set `PG_MAX_POOL_SIZE=100` → verify pool uses 100
- Without env var → verify auto-scales to 80% of available
