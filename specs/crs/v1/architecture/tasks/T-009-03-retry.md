# T-009-03: Retry with Exponential Backoff

| Field | Value |
|---|---|
| **Task ID** | T-009-03 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/common/resilience/retry.go` |
| **Type** | New file |

---

## Objective

Structured retry with exponential backoff + jitter. Replaces fixed `time.Sleep(100ms)` delays in DB reconnection and webhook delivery.

## Implementation

```go
package resilience

type RetryConfig struct {
    MaxRetries   int           // default: 3
    InitialDelay time.Duration // default: 100ms
    MaxDelay     time.Duration // default: 30s
    Multiplier   float64       // default: 2.0
    Jitter       bool          // default: true
}

// Retry executes fn with exponential backoff.
// Delays: 100ms → 200ms → 400ms → ... → capped at MaxDelay.
// Jitter: 50-100% of calculated delay to prevent thundering herd.
func Retry(ctx context.Context, name string, cfg RetryConfig, fn func(context.Context) error) error

// Prometheus: bytebase_retry_total{operation, attempt}
```

## Acceptance Criteria

- [ ] Exponential backoff with configurable multiplier
- [ ] Jitter randomizes delay by 50-100%
- [ ] Context cancellation stops retries immediately
- [ ] Prometheus counter per attempt
- [ ] Unit test: verify increasing delays, max cap, jitter range
