# T-009-01: Circuit Breaker Library

| Field | Value |
|---|---|
| **Task ID** | T-009-01 |
| **Solution** | SOL-ARCH-009 |
| **Priority** | P0 |
| **Depends On** | None |
| **Target File** | `backend/common/resilience/circuit_breaker.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Implement circuit breaker pattern: Closed→Open→HalfOpen state machine. Stops requests to failing dependencies after N consecutive failures. Includes Prometheus metrics.

## Implementation — DELIVERED

### File: `backend/common/resilience/circuit_breaker.go` (181 lines)

### State Machine

```
StateClosed ──(N failures)──→ StateOpen ──(resetTimeout)──→ StateHalfOpen
     ↑                                                          │
     └──────(probe succeeds)───────────────────────────────────┘
```

### Types

```go
type State int
const (
    StateClosed  State = iota  // normal operation
    StateOpen                   // tripped, fast-fail
    StateHalfOpen               // recovery, one probe allowed
)

type CircuitBreakerConfig struct {
    Name         string
    MaxFailures  int           // default: 5
    ResetTimeout time.Duration // default: 30s
}

type CircuitBreaker struct { /* mutex, state, failure count, timers */ }
type ErrCircuitOpen struct { Name string; RetryAfter time.Duration }
```

### Key Functions

| Function | Description |
|----------|-------------|
| `NewCircuitBreaker(cfg)` | Creates CB with defaults |
| `Execute(ctx, fn)` | Runs `fn` through state machine: Open→fast-fail, HalfOpen→probe, Closed→normal |
| `State.String()` | String representation for logging |
| `ErrCircuitOpen.Error()` | Descriptive error with retry-after info |

### Prometheus Metrics

- `bytebase_circuit_breaker_state{name}` — gauge (0=Closed, 1=Open, 2=HalfOpen)
- `bytebase_circuit_breaker_failures_total{name}` — counter
- `bytebase_circuit_breaker_rejected_total{name}` — counter

## Acceptance Criteria

- [x] `CircuitBreaker` with Closed/Open/HalfOpen states ✅
- [x] `Execute()` respects state transitions ✅
- [x] Prometheus metrics registered ✅
- [x] Unit tests pass (12 tests total across resilience package) ✅
- [x] `go build ./backend/common/resilience/...` passes ✅

## Verification

```
$ go build ./backend/common/resilience/... → ✅ PASS
$ go test ./backend/common/resilience/... → ok (2.483s) ✅
$ wc -l backend/common/resilience/circuit_breaker.go → 181
```
