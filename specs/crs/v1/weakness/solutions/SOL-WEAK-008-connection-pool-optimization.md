# Solution: Connection Pool Optimization — CR-WEAK-008

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-008                                             |
| **CR Reference**   | CR-WEAK-008                                              |
| **Title**          | Dual Pool Architecture + Observability + Reconnect Fix   |
| **Affected Layers**| L8 (Store), L10 (Infrastructure), L6 (Runner)            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per TDD.md §4.1: `Store` wraps `DBConnectionManager` with single `*sql.DB`. HA mode disables caches but all traffic goes through same pool.

Per architecture.md §7 (L6): 8 background runners share pool with API requests. Runners include heavy operations: schema sync, task execution, data cleanup.

**Verified** (db_connection.go:242-244):
```go
maxOpenConns := maxConns - reservedConns
if maxOpenConns > 50 { maxOpenConns = 50 }  // HARD CAP
```

**Reconnection** (db_connection.go:137-178): `time.Sleep(100ms)` race condition + 1-hour force close.

---

## 2. Architectural Change: Dual Pool Manager

### Current Architecture

```
                  ┌─────────────┐
All traffic ──────► *sql.DB     │  50 conn cap
(API + Runners)   │ (single)    │
                  └──────┬──────┘
                         │
                  ┌──────▼──────┐
                  │ PostgreSQL  │
                  └─────────────┘
```

### Proposed Architecture

```
                  ┌──────────────────────────────────┐
                  │      PoolManager                 │
                  │                                   │
API requests ────►│  apiPool    *sql.DB  (70% conns) │
                  │                                   │
Runners     ────►│  runnerPool *sql.DB  (30% conns) │
                  │                                   │
                  │  metrics   PoolMetrics            │
                  └───────────┬──────────────────────┘
                              │
                  ┌───────────▼───────────┐
                  │     PostgreSQL        │
                  │  (configurable pool)  │
                  └───────────────────────┘
```

---

## 3. Solution Design

### 3.1 Pool Manager (Replaces DBConnectionManager)

**New file**: `backend/store/pool_manager.go`

```go
package store

import (
    "context"
    "database/sql"
    "log/slog"
    "os"
    "strconv"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
)

// PoolConfig holds connection pool configuration.
type PoolConfig struct {
    MaxConnections  int     // PG_MAX_CONNECTIONS, 0 = auto-detect
    APIPoolRatio    float64 // PG_API_POOL_RATIO, default 0.7
    DrainTimeout    time.Duration // PG_POOL_DRAIN_TIMEOUT, default 5m
}

func LoadPoolConfig() PoolConfig {
    cfg := PoolConfig{
        APIPoolRatio: 0.7,
        DrainTimeout: 5 * time.Minute,
    }
    if v := os.Getenv("PG_MAX_CONNECTIONS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { cfg.MaxConnections = n }
    }
    if v := os.Getenv("PG_API_POOL_RATIO"); v != "" {
        if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f < 1 {
            cfg.APIPoolRatio = f
        }
    }
    if v := os.Getenv("PG_POOL_DRAIN_TIMEOUT"); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            cfg.DrainTimeout = time.Duration(n) * time.Second
        }
    }
    return cfg
}

// PoolManager manages dual connection pools for API and runner workloads.
type PoolManager struct {
    mu         sync.RWMutex
    apiPool    *sql.DB
    runnerPool *sql.DB
    config     PoolConfig
    pgURL      string
    metrics    *poolMetrics
}

// PoolType selects which pool to use.
type PoolType int
const (
    PoolAPI    PoolType = iota
    PoolRunner
)

func NewPoolManager(pgURL string, config PoolConfig) *PoolManager {
    return &PoolManager{
        pgURL:   pgURL,
        config:  config,
        metrics: newPoolMetrics(),
    }
}

func (pm *PoolManager) Initialize(ctx context.Context) error {
    // Detect PG max_connections
    probeDB, err := createConnection(ctx, pm.pgURL)
    if err != nil { return err }
    
    maxConns, reservedConns := detectPGLimits(probeDB)
    probeDB.Close()
    
    // Calculate effective pool size
    effectiveMax := pm.config.MaxConnections
    if effectiveMax == 0 {
        effectiveMax = int(float64(maxConns - reservedConns) * 0.7)
    }
    if effectiveMax < 10 { effectiveMax = 10 }
    if effectiveMax > 200 { effectiveMax = 200 }

    apiConns := int(float64(effectiveMax) * pm.config.APIPoolRatio)
    runnerConns := effectiveMax - apiConns
    if runnerConns < 5 { runnerConns = 5 }

    slog.Info("Connection pool configuration",
        slog.Int("pg_max_connections", maxConns),
        slog.Int("effective_pool", effectiveMax),
        slog.Int("api_pool", apiConns),
        slog.Int("runner_pool", runnerConns),
    )

    // Create dual pools
    pm.apiPool, err = createConnectionWithTracer(ctx, pm.pgURL)
    if err != nil { return err }
    pm.apiPool.SetMaxOpenConns(apiConns)
    pm.apiPool.SetMaxIdleConns(apiConns / 2)
    pm.apiPool.SetConnMaxLifetime(30 * time.Minute)

    pm.runnerPool, err = createConnectionWithTracer(ctx, pm.pgURL)
    if err != nil {
        pm.apiPool.Close()
        return err
    }
    pm.runnerPool.SetMaxOpenConns(runnerConns)
    pm.runnerPool.SetMaxIdleConns(runnerConns / 2)
    pm.runnerPool.SetConnMaxLifetime(30 * time.Minute)

    // Start metrics collection
    go pm.collectMetrics(ctx)

    return nil
}

// GetDB returns the appropriate pool for the given pool type.
func (pm *PoolManager) GetDB(pt PoolType) *sql.DB {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    if pt == PoolRunner {
        return pm.runnerPool
    }
    return pm.apiPool
}

// GetDefaultDB returns the API pool (backward compatible).
func (pm *PoolManager) GetDefaultDB() *sql.DB {
    return pm.GetDB(PoolAPI)
}

func (pm *PoolManager) Close() error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    var errs []error
    if pm.apiPool != nil { errs = append(errs, pm.apiPool.Close()) }
    if pm.runnerPool != nil { errs = append(errs, pm.runnerPool.Close()) }
    // return first error
    for _, e := range errs { if e != nil { return e } }
    return nil
}
```

### 3.2 Pool Prometheus Metrics

**Embedded in pool_manager.go**:

```go
type poolMetrics struct {
    activeConns  *prometheus.GaugeVec
    idleConns    *prometheus.GaugeVec
    waitCount    *prometheus.GaugeVec
    maxConns     *prometheus.GaugeVec
    waitDuration *prometheus.HistogramVec
}

func newPoolMetrics() *poolMetrics {
    m := &poolMetrics{
        activeConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_active_connections",
        }, []string{"pool"}),
        idleConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_idle_connections",
        }, []string{"pool"}),
        waitCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_waiting_requests",
        }, []string{"pool"}),
        maxConns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_db_pool_max_connections",
        }, []string{"pool"}),
    }
    prometheus.MustRegister(m.activeConns, m.idleConns, m.waitCount, m.maxConns)
    return m
}

func (pm *PoolManager) collectMetrics(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            for _, p := range []struct{ name string; db *sql.DB }{
                {"api", pm.apiPool}, {"runner", pm.runnerPool},
            } {
                stats := p.db.Stats()
                pm.metrics.activeConns.WithLabelValues(p.name).Set(float64(stats.InUse))
                pm.metrics.idleConns.WithLabelValues(p.name).Set(float64(stats.Idle))
                pm.metrics.waitCount.WithLabelValues(p.name).Set(float64(stats.WaitCount))
                pm.metrics.maxConns.WithLabelValues(p.name).Set(float64(stats.MaxOpenConnections))
            }
        }
    }
}
```

### 3.3 Robust Reconnection (Replaces time.Sleep)

```go
func (pm *PoolManager) reloadConnection(ctx context.Context, filePath string) {
    // File stability check — read twice with interval
    var newURL string
    for retries := 0; retries < 5; retries++ {
        url1, err := readURLFromFile(filePath)
        if err != nil || url1 == "" {
            time.Sleep(50 * time.Millisecond)
            continue
        }
        time.Sleep(50 * time.Millisecond)
        url2, _ := readURLFromFile(filePath)
        if url1 == url2 {
            newURL = url1
            break
        }
    }
    if newURL == "" {
        slog.Error("Failed to read stable PG URL from file", "file", filePath)
        return
    }

    // Create new pools
    newAPI, err := createConnectionWithTracer(ctx, newURL)
    if err != nil { slog.Error("reconnect: failed to create API pool", "error", err); return }
    newRunner, err := createConnectionWithTracer(ctx, newURL)
    if err != nil { newAPI.Close(); slog.Error("reconnect: failed to create runner pool", "error", err); return }

    // Configure same as Initialize
    // ...set MaxOpenConns, MaxIdleConns...

    // Swap atomically
    pm.mu.Lock()
    oldAPI, oldRunner := pm.apiPool, pm.runnerPool
    pm.apiPool, pm.runnerPool = newAPI, newRunner
    pm.mu.Unlock()

    // Drain old pools with context timeout (NOT 1 hour)
    drainCtx, cancel := context.WithTimeout(ctx, pm.config.DrainTimeout)
    go func() {
        defer cancel()
        <-drainCtx.Done()
        if oldAPI != nil { oldAPI.Close() }
        if oldRunner != nil { oldRunner.Close() }
        slog.Info("Old connection pools drained and closed")
    }()

    reconnectCounter.Inc()
    slog.Info("Database connection pools updated", "file", filePath)
}
```

### 3.4 Store Integration

**Modified**: `backend/store/store.go`

```go
type Store struct {
    poolManager *PoolManager  // REPLACES dbConnManager
    enableCache bool
    // ...caches unchanged...
}

func New(ctx context.Context, pgURL string, enableCache bool) (*Store, error) {
    // ...cache init unchanged...

    poolCfg := LoadPoolConfig()
    pm := NewPoolManager(pgURL, poolCfg)
    if err := pm.Initialize(ctx); err != nil {
        return nil, err
    }

    s := &Store{
        poolManager: pm,
        enableCache: enableCache,
        // ...caches...
    }
    return s, nil
}

// GetDB returns the API pool (backward compatible for existing callers).
func (s *Store) GetDB() *sql.DB {
    return s.poolManager.GetDefaultDB()
}

// GetRunnerDB returns the runner pool for background operations.
func (s *Store) GetRunnerDB() *sql.DB {
    return s.poolManager.GetDB(PoolRunner)
}
```

### 3.5 Runner Integration

**Modified**: `backend/runner/taskrun/scheduler.go` (and other runners)

Runners use `store.GetRunnerDB()` instead of `store.GetDB()`:

```go
// In runner initialization, store runner-specific DB reference
type Scheduler struct {
    store *store.Store
    // Runners call store.GetRunnerDB() for their queries
}
```

The `Store` methods internally use `GetDB()` (API pool). For runner-specific operations that bypass Store methods (e.g., direct SQL in migration executor), pass `GetRunnerDB()` explicitly.

---

## 4. Architecture Impact

### New Dependency Flow

```
L2 (server.go)
  └─ store.New() → PoolManager.Initialize()
       ├─ apiPool    ← used by L3, L4 (via store methods)
       └─ runnerPool ← used by L6 (via store.GetRunnerDB())

L6 (Runners)
  └─ store.GetRunnerDB() → runnerPool
       Isolated from API traffic congestion

L4 (Services)
  └─ store.GetDB() → apiPool (default)
       Priority for user-facing requests
```

### Breaking Changes

- `DBConnectionManager` replaced by `PoolManager` — internal only, no public API change
- `store.GetDB()` unchanged (returns apiPool) — backward compatible
- New `store.GetRunnerDB()` — opt-in for runners

---

## 5. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/store/pool_manager.go` | L8 | NEW | Dual pool + metrics + reconnect |
| `backend/store/store.go` | L8 | MODIFY | Use PoolManager, add GetRunnerDB() |
| `backend/store/db_connection.go` | L8 | DEPRECATE | Replaced by pool_manager.go |
| `backend/server/server.go` | L2 | MODIFY | Pass pool config to store.New() |
| `backend/runner/taskrun/scheduler.go` | L6 | MODIFY | Use GetRunnerDB() |
| `backend/runner/schemasync/syncer.go` | L6 | MODIFY | Use GetRunnerDB() |
| `backend/component/config/profile.go` | L10 | MODIFY | Add pool config env vars |

## 6. Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `PG_MAX_CONNECTIONS` | `0` (auto) | Total pool size (0 = 70% of PG max) |
| `PG_API_POOL_RATIO` | `0.7` | Fraction for API pool |
| `PG_POOL_DRAIN_TIMEOUT` | `300` | Seconds to drain old pool |

## 7. Test Strategy

```go
func TestPoolManager_DualPool(t *testing.T) {
    // API pool and runner pool are separate sql.DB instances
}

func TestPoolManager_AutoDetect(t *testing.T) {
    // PG_MAX_CONNECTIONS=0 → auto from PG max_connections
}

func TestPoolManager_Reconnect_FileStability(t *testing.T) {
    // Write file partially → verify retries until stable
}

func TestPoolManager_MetricsExported(t *testing.T) {
    // Verify Prometheus metrics for both pools
}
```

## 8. Rollback

- Set `PG_API_POOL_RATIO=1.0` → runner pool gets 0 conns, falls back to API pool
- Or revert to `DBConnectionManager` — swap in store.go constructor
- No database migration → no data rollback
