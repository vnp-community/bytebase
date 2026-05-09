# T-009-02: Bulkhead Library

| Field | Value |
|---|---|
| **Task ID** | T-009-02 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/common/resilience/bulkhead.go` |
| **Type** | New file |

---

## Objective

Semaphore-based concurrency limiter. Prevents resource-intensive operations from consuming all available connections/goroutines.

## Implementation

```go
package resilience

type Bulkhead struct {
    name string
    sem  chan struct{}
}

func NewBulkhead(name string, maxConcurrent int) *Bulkhead

// Execute runs fn within concurrency limit.
// Blocks if at capacity, respects ctx cancellation.
func (b *Bulkhead) Execute(ctx context.Context, fn func(context.Context) error) error

// Prometheus metrics:
//   bytebase_bulkhead_active{name} — gauge
//   bytebase_bulkhead_queued{name} — gauge
//   bytebase_bulkhead_max{name}    — gauge (constant)
```

## Acceptance Criteria

- [ ] `Bulkhead` with semaphore-based limiting
- [ ] Context cancellation while queued returns error
- [ ] Prometheus metrics: active, queued, max
- [ ] Unit test: 10 concurrent with limit 3 → max 3 active
- [ ] `go build ./backend/common/resilience/...` passes
