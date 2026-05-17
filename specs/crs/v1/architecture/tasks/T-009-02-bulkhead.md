# T-009-02: Bulkhead Library

| Field | Value |
|---|---|
| **Task ID** | T-009-02 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/common/resilience/bulkhead.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Semaphore-based concurrency limiter. Prevents resource-intensive operations from consuming all available connections/goroutines.

## Implementation — DELIVERED

### File: `backend/common/resilience/bulkhead.go` (74 lines)

### Design

```go
type Bulkhead struct {
    name string
    sem  chan struct{}   // buffered channel as semaphore
}

type ErrBulkheadTimeout struct {
    Name string
    Err  error
}

func NewBulkhead(name string, maxConcurrent int) *Bulkhead
func (b *Bulkhead) Execute(ctx context.Context, fn func(context.Context) error) error
```

### Execution Flow

```
Execute(ctx, fn):
  1. select on sem <- struct{}{} (acquire slot) vs ctx.Done() (cancelled)
  2. If slot acquired → run fn(ctx) → release slot
  3. If ctx cancelled → return ErrBulkheadTimeout
```

- Channel-based semaphore — zero-allocation when under capacity
- Context-aware — cancellation while queued returns immediately
- `ErrBulkheadTimeout` wraps the context error for structured handling

### Prometheus Metrics

- `bytebase_bulkhead_active{name}` — gauge (current active executions)
- `bytebase_bulkhead_queued{name}` — gauge (waiting for slot)
- `bytebase_bulkhead_max{name}` — gauge (constant, configured capacity)

## Acceptance Criteria

- [x] `Bulkhead` with semaphore-based limiting ✅
- [x] Context cancellation while queued returns error ✅
- [x] Prometheus metrics: active, queued, max ✅
- [x] Unit tests pass ✅
- [x] `go build ./backend/common/resilience/...` passes ✅

## Verification

```
$ go build ./backend/common/resilience/... → ✅ PASS
$ go test ./backend/common/resilience/... → ok (2.483s) ✅
$ wc -l backend/common/resilience/bulkhead.go → 74
```
