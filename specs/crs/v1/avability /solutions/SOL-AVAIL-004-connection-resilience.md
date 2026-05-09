# Solution: Database Connection Resilience & Connection Pool HA

| Field          | Value                                    |
|----------------|------------------------------------------|
| **Solution ID**| SOL-AVAIL-004                            |
| **CR ID**      | CR-AVAIL-004                             |
| **Status**     | Draft                                    |
| **Created**    | 2026-05-08                               |
| **Layers**     | L7 (Plugin), L8 (Store)                  |

---

## 1. Analysis — Existing Infrastructure

### 1.1 Điểm tận dụng

| Component | File | Capability |
|---|---|---|
| **DBConnectionManager** | `backend/store/db_connection.go` | PG connection via `database/sql` + pgx driver |
| **Store constructor** | `backend/store/store.go:43` | `New(ctx, pgURL, enableCache)` — fixed config |
| **DB metrics** | `backend/store/db_metrics.go` | Existing metrics collection |
| **DB tracer** | `backend/store/db_tracer.go` | SQL query timing |
| **Driver Plugin** | `backend/plugin/db/` | `db.Driver` interface with `Ping()`, `Open()`, `Close()` |
| **DBFactory** | `backend/component/dbfactory/` | Connection factory for managed instances |
| **Advisory Lock** | `backend/store/advisory_lock.go` | Uses `db.Conn(ctx)` — dedicated connections |

### 1.2 Key Findings from Source Code

```go
// store/store.go:80 — Connection setup
dbConnManager := NewDBConnectionManager(pgURL)
// → uses database/sql default pool settings
// → No MaxOpenConns, MaxIdleConns, ConnMaxLifetime configured!

// store/store.go:140 — HA mode disables cache
// → Every request goes to DB in HA mode
// → Pool pressure significantly higher than single-node
```

**Critical gap**: `database/sql` default pool has no limits → can open unlimited connections. In HA mode with cache disabled, this is a severe risk.

---

## 2. Giải pháp kỹ thuật

### 2.1 Configurable Connection Pool

**File**: `backend/store/db_connection.go` — Modify existing `DBConnectionManager`.

```go
// db_connection.go — Enhanced with configurable pool

// PoolConfig holds connection pool settings.
type PoolConfig struct {
    MaxOpenConns    int           // Default: 50
    MaxIdleConns    int           // Default: 10
    ConnMaxLifetime time.Duration // Default: 1h
    ConnMaxIdleTime time.Duration // Default: 15min
}

// DefaultPoolConfig returns sensible defaults.
func DefaultPoolConfig() PoolConfig {
    numCPU := runtime.NumCPU()
    return PoolConfig{
        MaxOpenConns:    max(50, numCPU*10),
        MaxIdleConns:    max(10, numCPU*2),
        ConnMaxLifetime: time.Hour,
        ConnMaxIdleTime: 15 * time.Minute,
    }
}

// PoolConfigFromEnv reads pool config from environment variables.
func PoolConfigFromEnv() PoolConfig {
    cfg := DefaultPoolConfig()
    if v := os.Getenv("PG_POOL_MAX_CONNS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.MaxOpenConns = n
        }
    }
    if v := os.Getenv("PG_POOL_MAX_IDLE_CONNS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.MaxIdleConns = n
        }
    }
    if v := os.Getenv("PG_POOL_MAX_LIFETIME"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.ConnMaxLifetime = d
        }
    }
    if v := os.Getenv("PG_POOL_MAX_IDLE_TIME"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.ConnMaxIdleTime = d
        }
    }
    return cfg
}

func (m *DBConnectionManager) Initialize(ctx context.Context) error {
    // ... existing connection logic ...
    
    // Apply pool configuration
    cfg := PoolConfigFromEnv()
    db := m.GetDB()
    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)
    db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
    
    slog.Info("Connection pool configured",
        slog.Int("maxOpen", cfg.MaxOpenConns),
        slog.Int("maxIdle", cfg.MaxIdleConns),
        slog.Duration("maxLifetime", cfg.ConnMaxLifetime),
        slog.Duration("maxIdleTime", cfg.ConnMaxIdleTime))
    
    return nil
}
```

### 2.2 Pool Metrics Exporter

**File**: `backend/store/db_metrics.go` — Enhance existing metrics.

```go
// db_metrics.go — Enhanced with pool stats

type PoolMetricsCollector struct {
    db *sql.DB
    
    // Gauges
    openConns      prometheus.Gauge
    inUseConns     prometheus.Gauge
    idleConns      prometheus.Gauge
    maxOpenConns   prometheus.Gauge
    waitCount      prometheus.Counter
    waitDuration   prometheus.Counter
}

func NewPoolMetricsCollector(db *sql.DB, registry prometheus.Registerer) *PoolMetricsCollector {
    c := &PoolMetricsCollector{
        db: db,
        openConns: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_open_connections",
            Help: "Number of open connections",
        }),
        inUseConns: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_in_use_connections",
            Help: "Number of connections in use",
        }),
        idleConns: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_idle_connections",
            Help: "Number of idle connections",
        }),
        maxOpenConns: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_max_open_connections",
            Help: "Maximum number of open connections",
        }),
        waitCount: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bytebase_db_pool_wait_total",
            Help: "Total number of waits for connection from pool",
        }),
        waitDuration: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bytebase_db_pool_wait_duration_seconds_total",
            Help: "Total wait time for connections from pool",
        }),
    }
    registry.MustRegister(
        c.openConns, c.inUseConns, c.idleConns,
        c.maxOpenConns, c.waitCount, c.waitDuration,
    )
    return c
}

// Collect reads sql.DBStats and updates Prometheus gauges.
func (c *PoolMetricsCollector) Collect() {
    stats := c.db.Stats()
    c.openConns.Set(float64(stats.OpenConnections))
    c.inUseConns.Set(float64(stats.InUse))
    c.idleConns.Set(float64(stats.Idle))
    c.maxOpenConns.Set(float64(stats.MaxOpenConnections))
    // WaitCount and WaitDuration are cumulative in sql.DBStats
}
```

### 2.3 Retry Engine for Store Operations

**File**: `backend/store/retry.go` (NEW)

```go
package store

import (
    "context"
    "math"
    "math/rand"
    "time"
    
    "github.com/jackc/pgx/v5/pgconn"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
    MaxAttempts int           // Default: 3
    BaseDelay   time.Duration // Default: 500ms
    MaxDelay    time.Duration // Default: 30s
    JitterRatio float64       // Default: 0.2 (20%)
}

var DefaultRetryConfig = RetryConfig{
    MaxAttempts: 3,
    BaseDelay:   500 * time.Millisecond,
    MaxDelay:    30 * time.Second,
    JitterRatio: 0.2,
}

// RetryableExec executes fn with retry for transient PostgreSQL errors.
func RetryableExec(ctx context.Context, cfg RetryConfig, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt <= cfg.MaxAttempts; attempt++ {
        lastErr = fn()
        if lastErr == nil {
            return nil
        }
        
        if !isRetryable(lastErr) {
            return lastErr
        }
        
        if attempt == cfg.MaxAttempts {
            break
        }
        
        delay := cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt)))
        if delay > cfg.MaxDelay {
            delay = cfg.MaxDelay
        }
        // Add jitter
        jitter := time.Duration(float64(delay) * cfg.JitterRatio * (rand.Float64()*2 - 1))
        delay += jitter
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
        }
        
        slog.Debug("Retrying DB operation",
            slog.Int("attempt", attempt+1),
            slog.String("error", lastErr.Error()))
    }
    
    return fmt.Errorf("max retries exceeded (%d): %w", cfg.MaxAttempts, lastErr)
}

// isRetryable checks if a PostgreSQL error is transient and retryable.
func isRetryable(err error) bool {
    if err == nil {
        return false
    }
    
    // Check pgx-specific error codes
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "40001": // serialization_failure
            return true
        case "40P01": // deadlock_detected
            return true
        case "55P03": // lock_not_available
            return true
        case "57P01": // admin_shutdown
            return true
        case "57P02": // crash_shutdown
            return true
        case "57P03": // cannot_connect_now
            return true
        case "08000": // connection_exception
            return true
        case "08003": // connection_does_not_exist
            return true
        case "08006": // connection_failure
            return true
        }
    }
    
    // Check for connection-level errors
    errStr := err.Error()
    retryableErrors := []string{
        "connection refused",
        "connection reset",
        "broken pipe",
        "i/o timeout",
        "no such host",
        "connection timed out",
    }
    for _, pattern := range retryableErrors {
        if strings.Contains(strings.ToLower(errStr), pattern) {
            return true
        }
    }
    
    return false
}
```

### 2.4 Retry Wrapper for Plugin Drivers

**File**: `backend/plugin/db/retry_wrapper.go` (NEW)

```go
package db

import (
    "context"
)

// RetryDriver wraps a Driver with automatic retry for transient errors.
type RetryDriver struct {
    inner     Driver
    retryCfg  store.RetryConfig
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

// Passthrough methods (no retry needed for these)
func (d *RetryDriver) Open(ctx context.Context, dbType storepb.Engine, config ConnectionConfig) (Driver, error) {
    return d.inner.Open(ctx, dbType, config)
}
func (d *RetryDriver) Close(ctx context.Context) error { return d.inner.Close(ctx) }
func (d *RetryDriver) GetDB() *sql.DB                  { return d.inner.GetDB() }

// QueryConn — retry only on connection errors, not on query errors
func (d *RetryDriver) QueryConn(ctx context.Context, conn *sql.Conn, stmt string, qc *QueryContext) ([]*QueryResult, error) {
    // Don't retry SELECT queries (idempotent but may have side effects)
    return d.inner.QueryConn(ctx, conn, stmt, qc)
}
```

### 2.5 DBFactory Integration

**File**: `backend/component/dbfactory/dbfactory.go` — Wrap drivers with retry.

```go
// dbfactory.go — In the driver creation path
func (f *DBFactory) GetDriver(ctx context.Context, instance *store.InstanceMessage, database string) (db.Driver, error) {
    driver, err := f.getDriverInternal(ctx, instance, database)
    if err != nil {
        return nil, err
    }
    
    // Wrap with retry for managed database operations
    retryCfg := store.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    10 * time.Second,
        JitterRatio: 0.2,
    }
    
    return db.NewRetryDriver(driver, retryCfg), nil
}
```

### 2.6 Connection Health Background Runner

**Approach**: Extend existing `MemoryMonitor` pattern to include pool stats.

**File**: `backend/runner/monitor/pool_monitor.go` (NEW)

```go
package monitor

// PoolMonitor periodically checks connection pool stats and alerts on issues.
type PoolMonitor struct {
    db          *sql.DB
    metricsCol  *store.PoolMetricsCollector
}

func (m *PoolMonitor) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            m.check(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (m *PoolMonitor) check(ctx context.Context) {
    stats := m.db.Stats()
    m.metricsCol.Collect()
    
    utilization := float64(stats.InUse) / float64(stats.MaxOpenConnections)
    
    if utilization > 0.8 {
        slog.Warn("DB pool high utilization",
            slog.Float64("utilization", utilization),
            slog.Int("inUse", stats.InUse),
            slog.Int("maxOpen", stats.MaxOpenConnections))
    }
    
    if stats.WaitCount > 0 {
        slog.Debug("DB pool wait stats",
            slog.Int64("waitCount", stats.WaitCount),
            slog.Duration("waitDuration", stats.WaitDuration))
    }
}
```

---

## 3. File Change Summary

| Layer | File | Change Type | Description |
|---|---|---|---|
| L8 | `backend/store/db_connection.go` | **Modify** | `PoolConfig`, `PoolConfigFromEnv()`, apply pool settings |
| L8 | `backend/store/db_metrics.go` | **Modify** | `PoolMetricsCollector` with sql.DBStats |
| L8 | `backend/store/retry.go` | **New** | `RetryableExec`, `isRetryable` for PG errors |
| L7 | `backend/plugin/db/retry_wrapper.go` | **New** | `RetryDriver` wrapping `db.Driver` |
| L5 | `backend/component/dbfactory/dbfactory.go` | **Modify** | Wrap drivers with `RetryDriver` |
| L6 | `backend/runner/monitor/pool_monitor.go` | **New** | Pool utilization monitoring |
| L2 | `backend/server/server.go` | **Modify** | Wire pool monitor runner |

---

## 4. Backward Compatibility

| Scenario | Behavior |
|---|---|
| No env vars set | `DefaultPoolConfig()` applies sensible defaults (50 max, 10 idle) |
| Existing `database/sql` usage | Pool settings applied after initialization — all existing code benefits |
| Retry wrapper | Transparent — same `db.Driver` interface, callers unchanged |
| Advisory locks | `db.Conn(ctx)` counts against MaxOpenConns — accounted in defaults |

---

## 5. Key Design Decisions

1. **Use `database/sql` pool, not custom pool**: Bytebase already uses `database/sql` via pgx. Adding pool settings to `*sql.DB` is the least invasive change.
2. **Retry at Store level, not Service level**: Store (L8) is where SQL errors originate. Retrying here prevents duplicate retry logic across 30+ services.
3. **RetryDriver wraps Driver, not replaces**: Plugin interface (`db.Driver`) preserved. `RetryDriver` is a decorator — no plugin changes needed.
4. **Pool monitor is a separate runner**: Follows existing pattern (`MemoryMonitor`, `DataCleaner`). Clean separation.
