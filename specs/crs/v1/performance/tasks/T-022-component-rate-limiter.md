# T-022: Component — Rate Limiter

| Field | Value |
|-------|-------|
| **Task ID** | T-022 |
| **Solution** | SOL-PERF-004 |
| **Type** | New file |
| **Priority** | P0 |
| **Depends on** | None |
| **Blocks** | T-024 |
| **Status** | DONE |

## Target File

`backend/component/ratelimit/limiter.go` (new)

## Implementation

```go
package ratelimit

import (
    "sync"
    "golang.org/x/time/rate"
)

type Config struct {
    ReadRatePerSecond  float64
    WriteRatePerSecond float64
    ReadBurst          int
    WriteBurst         int
}

var DefaultConfig = Config{
    ReadRatePerSecond: 1000, WriteRatePerSecond: 100,
    ReadBurst: 2000, WriteBurst: 200,
}

type WorkspaceLimiter struct {
    mu       sync.RWMutex
    limiters map[string]*rate.Limiter
    config   Config
}

func New(config Config) *WorkspaceLimiter {
    return &WorkspaceLimiter{
        limiters: make(map[string]*rate.Limiter),
        config:   config,
    }
}

func (wl *WorkspaceLimiter) Allow(workspace string, isWrite bool) bool {
    return wl.getOrCreateLimiter(workspace, isWrite).Allow()
}

func (wl *WorkspaceLimiter) getOrCreateLimiter(workspace string, isWrite bool) *rate.Limiter {
    key := workspace + ":read"
    r := rate.Limit(wl.config.ReadRatePerSecond)
    b := wl.config.ReadBurst
    if isWrite {
        key = workspace + ":write"
        r = rate.Limit(wl.config.WriteRatePerSecond)
        b = wl.config.WriteBurst
    }

    wl.mu.RLock()
    if l, ok := wl.limiters[key]; ok {
        wl.mu.RUnlock()
        return l
    }
    wl.mu.RUnlock()

    wl.mu.Lock()
    defer wl.mu.Unlock()
    if l, ok := wl.limiters[key]; ok { return l }
    l := rate.NewLimiter(r, b)
    wl.limiters[key] = l
    return l
}
```

## Dependency

Add `golang.org/x/time` to `go.mod` if not present.
