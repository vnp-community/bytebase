# T-003-03: Circuit Breaker Component

| Field | Value |
|---|---|
| **Task ID** | T-003-03 |
| **Solution** | SOL-AVAIL-003 |
| **Depends On** | None |
| **Target File** | `backend/component/circuitbreaker/breaker.go` (NEW) |

---

## Objective

Implement circuit breaker pattern (Closed → Open → HalfOpen) với Prometheus metrics. Generic, reusable cho bất kỳ external call.

## Implementation

Tạo file `backend/component/circuitbreaker/breaker.go` — xem full code tại SOL-AVAIL-003 §2.3.

Key API:
```go
b := circuitbreaker.New(Config{
    Name:             "instance_xyz",
    FailureThreshold: 5,   // 5 failures → open
    SuccessThreshold: 2,   // 2 successes in half-open → close
    Timeout:          30*time.Second, // wait before half-open
}, registry)

err := b.Execute(ctx, func(ctx context.Context) error {
    return driver.Ping(ctx)
})
// err == ErrCircuitOpen nếu circuit đang open
```

Key states:
- **Closed**: Normal, pass all requests. Open after `FailureThreshold` consecutive failures.
- **Open**: Reject all requests with `ErrCircuitOpen`. After `Timeout` → HalfOpen.
- **HalfOpen**: Allow requests. If `SuccessThreshold` successes → Closed. Any failure → Open.

## Acceptance Criteria

- [ ] State machine: Closed → Open → HalfOpen → Closed
- [ ] Thread-safe (`sync.RWMutex`)
- [ ] 3 Prometheus metrics: `state` (gauge), `failures_total` (counter), `rejected_total` (counter)
- [ ] `ErrCircuitOpen` sentinel error
- [ ] `go build ./backend/component/circuitbreaker/...` passes
