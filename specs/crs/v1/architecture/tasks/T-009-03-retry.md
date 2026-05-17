# T-009-03: Retry with Exponential Backoff

| Field | Value |
|---|---|
| **Task ID** | T-009-03 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/common/resilience/retry.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Structured retry with exponential backoff + jitter. Replaces fixed `time.Sleep(100ms)` delays in DB reconnection and webhook delivery.

## Implementation — DELIVERED

### File: `backend/common/resilience/retry.go` (104 lines)

### Configuration

```go
type RetryConfig struct {
    MaxRetries   int           // default: 3
    InitialDelay time.Duration // default: 100ms
    MaxDelay     time.Duration // default: 30s
    Multiplier   float64       // default: 2.0
    Jitter       bool          // default: true
}

var DefaultRetryConfig = RetryConfig{
    MaxRetries:   3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       true,
}
```

### Delay Progression

```
Attempt 1: 100ms × 1.0 = 100ms (± jitter)
Attempt 2: 100ms × 2.0 = 200ms (± jitter)
Attempt 3: 100ms × 4.0 = 400ms (± jitter)
Attempt 4: 100ms × 8.0 = 800ms (± jitter)
... capped at MaxDelay (30s)
```

### Jitter Algorithm

```
delay = calculatedDelay * (0.5 + rand.Float64()*0.5)
// Range: [50%, 100%] of calculated delay
// Prevents thundering herd on shared dependencies
```

### Key Function

```go
func Retry(ctx context.Context, name string, cfg RetryConfig, fn func(context.Context) error) error
```

- Context cancellation stops retries immediately
- Returns last error if all retries exhausted
- Prometheus counter: `bytebase_retry_total{operation, attempt}`

## Acceptance Criteria

- [x] Exponential backoff with configurable multiplier ✅
- [x] Jitter randomizes delay by 50-100% ✅
- [x] Context cancellation stops retries immediately ✅
- [x] Prometheus counter per attempt ✅
- [x] Unit tests pass ✅

## Verification

```
$ go build ./backend/common/resilience/... → ✅ PASS
$ go test ./backend/common/resilience/... → ok (2.483s) ✅
$ wc -l backend/common/resilience/retry.go → 104
```
