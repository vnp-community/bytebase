# T-020: Syncer — Adaptive Concurrency Pool

| Field | Value |
|-------|-------|
| **Task ID** | T-020 |
| **Solution** | SOL-PERF-003 |
| **Type** | New file |
| **Priority** | P1 |
| **Depends on** | None |
| **Blocks** | None |
| **Status** | DONE |

## Target File

`backend/runner/schemasync/adaptive_pool.go` (new)

## Implementation

```go
package schemasync

import (
    "os"
    "runtime"
    "strconv"
    "github.com/sourcegraph/conc/pool"
)

const (
    defaultMinConcurrency = 10
    defaultMaxConcurrency = 300
)

func (s *Syncer) newAdaptivePool() *pool.Pool {
    return pool.New().WithMaxGoroutines(calculateAdaptiveConcurrency())
}

func calculateAdaptiveConcurrency() int {
    if v := os.Getenv("SYNC_MAX_CONCURRENCY"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return n
        }
    }
    target := runtime.NumCPU() * 10
    if target < defaultMinConcurrency { return defaultMinConcurrency }
    if target > defaultMaxConcurrency { return defaultMaxConcurrency }
    return target
}
```

## Dependency

Add `github.com/sourcegraph/conc` to `go.mod` if not present.
