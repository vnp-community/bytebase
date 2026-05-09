# T-003-04: Wire Circuit Breaker into DBFactory

| Field | Value |
|---|---|
| **Task ID** | T-003-04 |
| **Solution** | SOL-AVAIL-003 |
| **Depends On** | T-003-03 |
| **Target File** | `backend/component/dbfactory/dbfactory.go` (Modify) |

---

## Objective

Thêm per-instance circuit breaker vào `DBFactory.GetDriver()`. Mỗi managed database instance có riêng 1 circuit breaker.

## Implementation

1. Add fields to DBFactory struct:
```go
breakers   map[string]*circuitbreaker.Breaker
breakersMu sync.RWMutex
registry   prometheus.Registerer
```

2. Wrap `GetDriver()`:
```go
func (f *DBFactory) GetDriver(ctx, instance, database) (db.Driver, error) {
    breaker := f.getOrCreateBreaker(instance.ResourceID)
    var driver db.Driver
    err := breaker.Execute(ctx, func(ctx context.Context) error {
        var err error
        driver, err = f.getDriverInternal(ctx, instance, database)
        return err
    })
    if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
        return nil, status.Errorf(codes.Unavailable,
            "instance %s unreachable (circuit breaker open)", instance.ResourceID)
    }
    return driver, err
}
```

3. `getOrCreateBreaker()` — double-checked locking pattern (see SOL-003 §2.4)

## Acceptance Criteria

- [ ] Per-instance circuit breaker (lazy-created, map key = ResourceID)
- [ ] `ErrCircuitOpen` → gRPC `codes.Unavailable`
- [ ] Existing `GetDriver` renamed to `getDriverInternal` (unexported)
- [ ] `go build ./backend/component/dbfactory/...` passes
