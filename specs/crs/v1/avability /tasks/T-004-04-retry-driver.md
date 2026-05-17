# T-004-04: Retry Driver Decorator

| Field | Value |
|---|---|
| **Task ID** | T-004-04 |
| **Solution** | SOL-AVAIL-004 |
| **Depends On** | T-004-02 |
| **Target File** | `backend/plugin/db/retry_wrapper.go` (NEW) |

---

## Objective

Tạo `RetryDriver` decorator wrapping `db.Driver` interface — retry tự động cho `Ping()` và `Execute()`. Transparent cho callers.

## Implementation

```go
package db

type RetryDriver struct {
    inner    Driver
    retryCfg store.RetryConfig
}

func NewRetryDriver(inner Driver, cfg store.RetryConfig) *RetryDriver {
    return &RetryDriver{inner: inner, retryCfg: cfg}
}

func (d *RetryDriver) Ping(ctx context.Context) error {
    return store.RetryableExec(ctx, d.retryCfg, func() error {
        return d.inner.Ping(ctx)
    })
}

func (d *RetryDriver) Execute(ctx context.Context, stmt string, opts ExecuteOptions) (int64, error) {
    var affected int64
    err := store.RetryableExec(ctx, d.retryCfg, func() error {
        var err error
        affected, err = d.inner.Execute(ctx, stmt, opts)
        return err
    })
    return affected, err
}

// Passthrough (no retry)
func (d *RetryDriver) Close(ctx context.Context) error { return d.inner.Close(ctx) }
func (d *RetryDriver) GetDB() *sql.DB                  { return d.inner.GetDB() }
```

> **Note**: Only retry Ping/Execute. Do NOT retry QueryConn (SELECT may have side-effect logging).

## Acceptance Criteria

- [x] Implements `db.Driver` interface (verify all methods)
- [x] Retry on Ping and Execute only
- [x] Passthrough for Close, GetDB, QueryConn
- [x] `go build ./backend/plugin/db/...` passes
