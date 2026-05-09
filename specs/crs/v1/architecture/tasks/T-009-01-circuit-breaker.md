# T-009-01: Circuit Breaker Library

| Field | Value |
|---|---|
| **Task ID** | T-009-01 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/common/resilience/circuit_breaker.go` |
| **Type** | New file |

---

## Objective

Implement circuit breaker pattern: Closed→Open→HalfOpen state machine. Stops requests to failing dependencies after N consecutive failures. Includes Prometheus metrics.

## Implementation

```go
package resilience

type State int
const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

type CircuitBreaker struct {
    mu           sync.Mutex
    name         string
    state        State
    failures     int
    maxFailures  int           // default: 5
    resetTimeout time.Duration // default: 30s
    lastFailure  time.Time
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker

// Execute runs fn through the circuit breaker.
// Open → returns error immediately (fast-fail).
// HalfOpen → allows one probe request.
// Closed → normal operation, counts failures.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error

// Prometheus metrics:
//   bytebase_circuit_breaker_state{name}         — gauge (0/1/2)
//   bytebase_circuit_breaker_failures_total{name} — counter
//   bytebase_circuit_breaker_rejected_total{name} — counter
```

## Acceptance Criteria

- [ ] `CircuitBreaker` with Closed/Open/HalfOpen states
- [ ] `Execute()` respects state transitions
- [ ] Prometheus metrics registered
- [ ] Unit tests: normal flow, trip, recovery, context cancellation
- [ ] `go build ./backend/common/resilience/...` passes
