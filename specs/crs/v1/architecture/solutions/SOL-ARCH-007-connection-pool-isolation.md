# Solution: Connection Pool Isolation — CR-ARCH-007

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-007                                             |
| **CR Reference**   | CR-ARCH-007                                              |
| **Title**          | Dual Pool Manager — API/Runner Isolation                 |
| **Affected Layers**| L8 (Store), L6 (Runner), L2 (Server)                     |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §8 (L8 — Connection Management):
- `DBConnectionManager` manages single `*sql.DB` instance
- `maxOpenConns` capped at 50 (db_connection.go:243-244)

Per [TDD.md](../../TDD.md) §4.1:
- Connection pool: `max_connections - superuser_reserved_connections`, capped at 50

---

## 2. Current Implementation Analysis

### 2.1 Single Pool (db_connection.go:242-246)

```go
maxOpenConns := maxConns - reservedConns
if maxOpenConns > 50 {
    maxOpenConns = 50
}
db.SetMaxOpenConns(maxOpenConns)
```

### 2.2 Shared Usage (server.go:262-287)

```go
// ALL runners and API services share the same *sql.DB pool
go s.taskScheduler.Run(ctx, &s.runnerWG)     // heavy: migration execution
go s.schemaSyncer.Run(ctx, &s.runnerWG)      // heavy: sync all DB schemas
// ... 6 more runners ...
// + 30+ API services all using stores.GetDB()
```

---

## 3. Solution Design

### 3.1 Pool Manager

**New file**: `backend/store/pool_manager.go`

```go
package store

import (
    "context"
    "database/sql"
    "fmt"
    "log/slog"

    "github.com/prometheus/client_golang/prometheus"
)

// PoolType identifies which connection pool to use.
type PoolType int

const (
    PoolAPI    PoolType = iota  // User-facing API requests
    PoolRunner                  // Background runner operations
)

// poolContextKey is the context key for pool routing.
type poolContextKey struct{}

// WithPool returns a context that routes DB queries to the specified pool.
func WithPool(ctx context.Context, pool PoolType) context.Context {
    return context.WithValue(ctx, poolContextKey{}, pool)
}

// PoolManager manages dual connection pools for API and Runner workloads.
type PoolManager struct {
    apiPool    *sql.DB
    runnerPool *sql.DB
    metrics    *poolMetrics
}

// PoolConfig configures the pool split ratios.
type PoolConfig struct {
    PGURL          string
    MaxConnections int  // Total connections available
    APIRatio       float64  // Fraction for API pool (e.g., 0.7)
    RunnerRatio    float64  // Fraction for runner pool (e.g., 0.3)
}

func NewPoolManager(ctx context.Context, cfg PoolConfig) (*PoolManager, error) {
    totalConns := cfg.MaxConnections
    if totalConns > 50 {
        totalConns = 50
    }

    apiConns := int(float64(totalConns) * cfg.APIRatio)
    runnerConns := totalConns - apiConns

    // Minimum 5 connections per pool
    if apiConns < 5 { apiConns = 5 }
    if runnerConns < 5 { runnerConns = 5 }

    slog.Info("Pool manager initialized",
        "total", totalConns,
        "api_pool", apiConns,
        "runner_pool", runnerConns,
    )

    apiPool, err := createConnectionWithTracer(ctx, cfg.PGURL)
    if err != nil {
        return nil, fmt.Errorf("create API pool: %w", err)
    }
    apiPool.SetMaxOpenConns(apiConns)
    apiPool.SetMaxIdleConns(apiConns / 2)

    runnerPool, err := createConnectionWithTracer(ctx, cfg.PGURL)
    if err != nil {
        apiPool.Close()
        return nil, fmt.Errorf("create runner pool: %w", err)
    }
    runnerPool.SetMaxOpenConns(runnerConns)
    runnerPool.SetMaxIdleConns(runnerConns / 2)

    pm := &PoolManager{
        apiPool:    apiPool,
        runnerPool: runnerPool,
        metrics:    newPoolMetrics(),
    }

    // Start metrics collection
    go pm.collectMetrics(ctx)

    return pm, nil
}

// GetDB returns the appropriate pool based on context.
// Default = API pool (safe default for user-facing requests).
func (pm *PoolManager) GetDB(ctx context.Context) *sql.DB {
    if pool, ok := ctx.Value(poolContextKey{}).(PoolType); ok {
        switch pool {
        case PoolRunner:
            return pm.runnerPool
        }
    }
    return pm.apiPool
}

// APIPool returns the API pool directly.
func (pm *PoolManager) APIPool() *sql.DB { return pm.apiPool }

// RunnerPool returns the runner pool directly.
func (pm *PoolManager) RunnerPool() *sql.DB { return pm.runnerPool }

func (pm *PoolManager) Close() error {
    if err := pm.apiPool.Close(); err != nil {
        return err
    }
    return pm.runnerPool.Close()
}

func (pm *PoolManager) collectMetrics(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            apiStats := pm.apiPool.Stats()
            pm.metrics.activeConns.WithLabelValues("api").Set(float64(apiStats.InUse))
            pm.metrics.idleConns.WithLabelValues("api").Set(float64(apiStats.Idle))
            pm.metrics.waitDuration.WithLabelValues("api").Set(apiStats.WaitDuration.Seconds())

            runnerStats := pm.runnerPool.Stats()
            pm.metrics.activeConns.WithLabelValues("runner").Set(float64(runnerStats.InUse))
            pm.metrics.idleConns.WithLabelValues("runner").Set(float64(runnerStats.Idle))
            pm.metrics.waitDuration.WithLabelValues("runner").Set(runnerStats.WaitDuration.Seconds())
        }
    }
}
```

### 3.2 Pool Metrics

**New file**: `backend/store/pool_metrics.go`

```go
package store

import "github.com/prometheus/client_golang/prometheus"

type poolMetrics struct {
    activeConns  *prometheus.GaugeVec
    idleConns    *prometheus.GaugeVec
    maxConns     *prometheus.GaugeVec
    waitDuration *prometheus.GaugeVec
}

func newPoolMetrics() *poolMetrics {
    m := &poolMetrics{
        activeConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_active_conns",
            Help: "Number of active database connections",
        }, []string{"pool"}),
        idleConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_idle_conns",
            Help: "Number of idle database connections",
        }, []string{"pool"}),
        maxConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_max_conns",
            Help: "Maximum database connections configured",
        }, []string{"pool"}),
        waitDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_wait_duration_seconds",
            Help: "Cumulative time blocked waiting for a connection",
        }, []string{"pool"}),
    }
    prometheus.MustRegister(m.activeConns, m.idleConns, m.maxConns, m.waitDuration)
    return m
}
```

### 3.3 Runner Integration

**Modified runner pattern** (example `runner/schemasync/syncer.go`):

```go
// BEFORE — uses shared Store.GetDB()
func (s *Syncer) syncInstance(ctx context.Context, instance *store.InstanceMessage) {
    // Uses default pool → competes with API
}

// AFTER — uses runner pool via context
func (s *Syncer) syncInstance(ctx context.Context, instance *store.InstanceMessage) {
    ctx = store.WithPool(ctx, store.PoolRunner)
    // All DB queries in this context use runner pool
}
```

### 3.4 Store Integration

**Modified file**: `backend/store/store.go`

```go
type Store struct {
    // OPTION A: Dual pool manager (new)
    poolManager *PoolManager
    // OPTION B: Single DB (legacy, backward compatible)
    dbConnManager *DBConnectionManager
    // ...
}

func (s *Store) GetDB() *sql.DB {
    if s.poolManager != nil {
        return s.poolManager.APIPool()  // default = API pool
    }
    return s.dbConnManager.GetDB()
}

func (s *Store) GetRunnerDB() *sql.DB {
    if s.poolManager != nil {
        return s.poolManager.RunnerPool()
    }
    return s.dbConnManager.GetDB()  // fallback to single pool
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/store/pool_manager.go` | L8 | **NEW** | Dual pool manager |
| `backend/store/pool_metrics.go` | L8 | **NEW** | Prometheus pool metrics |
| `backend/store/store.go` | L8 | **MODIFY** | Support PoolManager |
| `backend/runner/schemasync/syncer.go` | L6 | **MODIFY** | Use runner pool |
| `backend/runner/taskrun/scheduler.go` | L6 | **MODIFY** | Use runner pool |
| `backend/runner/cleaner/cleaner.go` | L6 | **MODIFY** | Use runner pool |
| `backend/server/server.go` | L2 | **MODIFY** | Init pool manager |
| `backend/component/config/profile.go` | L5 | **MODIFY** | Pool ratio config |

---

## 5. Rollback Plan

1. Set `PG_POOL_ISOLATION=false` → single pool (current behavior)
2. PoolManager wraps same PG URL — no data layer changes
