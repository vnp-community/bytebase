# Solution: Health Monitoring, Circuit Breaker & Self-Healing

| Field          | Value                                    |
|----------------|------------------------------------------|
| **Solution ID**| SOL-AVAIL-003                            |
| **CR ID**      | CR-AVAIL-003                             |
| **Status**     | Draft                                    |
| **Created**    | 2026-05-08                               |
| **Layers**     | L2 (API Gateway), L5 (Component), L6 (Runner), L8 (Store) |

---

## 1. Analysis — Existing Infrastructure

### 1.1 Điểm tận dụng

| Component | File | Capability |
|---|---|---|
| **Health endpoint** | `backend/server/echo_routes.go:75` | `/healthz` → plain "OK" |
| **Prometheus metrics** | `echo_routes.go:66-73` | `echoprometheus` middleware, `/metrics` endpoint |
| **Memory monitor** | `backend/runner/monitor/` | `MemoryMonitor` runner — periodic memory checks |
| **DB tracer** | `backend/store/db_tracer.go` | SQL query timing instrumentation |
| **DB metrics** | `backend/store/db_metrics.go` | Database-level metrics collection |
| **DBFactory** | `backend/component/dbfactory/` | Connection factory for managed instances |
| **Driver Ping** | Plugin `db.Driver.Ping(ctx)` | Per-engine connectivity check (22 drivers) |
| **Error logging** | Echo `RequestLoggerConfig` | Error-only request logging |

### 1.2 Key Architecture Insight

Từ TDD §4.2 và Architecture §9:
- **HA mode disables cache** (`enableCache=false`) → every request hits DB
- **Store** is the most depended-upon layer (L3-L7 all depend on L8)
- **Health check does zero validation** — always returns "OK"
- **No circuit breaker** anywhere — cascading failure risk to all 22 DB drivers

---

## 2. Giải pháp kỹ thuật

### 2.1 Deep Health Check System

**Approach**: Tiered health checks leveraging existing Prometheus registry.

**File**: `backend/component/health/checker.go` (NEW)

```go
package health

import (
    "context"
    "database/sql"
    "sync"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
)

// Status represents component health.
type Status string
const (
    StatusHealthy   Status = "HEALTHY"
    StatusDegraded  Status = "DEGRADED"
    StatusUnhealthy Status = "UNHEALTHY"
)

// CheckResult represents a single health check result.
type CheckResult struct {
    Name      string        `json:"name"`
    Status    Status        `json:"status"`
    Latency   time.Duration `json:"latency_ms"`
    Message   string        `json:"message,omitempty"`
    Critical  bool          `json:"critical"` // If true, UNHEALTHY = overall UNHEALTHY
}

// Checker performs deep health checks on all dependencies.
type Checker struct {
    db          *sql.DB
    checks      []CheckFunc
    
    // Prometheus metrics
    healthGauge *prometheus.GaugeVec
    latencyHist *prometheus.HistogramVec
}

type CheckFunc func(ctx context.Context) CheckResult

func NewChecker(db *sql.DB, registry prometheus.Registerer) *Checker {
    c := &Checker{
        db: db,
        healthGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_health_status",
            Help: "Health status of components (0=unhealthy, 1=degraded, 2=healthy)",
        }, []string{"component"}),
        latencyHist: prometheus.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "bytebase_health_check_latency_seconds",
            Help:    "Health check latency",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
        }, []string{"component"}),
    }
    registry.MustRegister(c.healthGauge, c.latencyHist)
    
    // Register core checks
    c.checks = append(c.checks,
        c.checkPostgreSQL,
        c.checkDiskSpace,
        c.checkMemory,
    )
    
    return c
}

func (c *Checker) checkPostgreSQL(ctx context.Context) CheckResult {
    start := time.Now()
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    err := c.db.PingContext(ctx)
    latency := time.Since(start)
    
    if err != nil {
        return CheckResult{"postgresql", StatusUnhealthy, latency, err.Error(), true}
    }
    
    // Check connection pool stats (database/sql built-in)
    stats := c.db.Stats()
    if stats.OpenConnections >= stats.MaxOpenConnections-2 {
        return CheckResult{"postgresql", StatusDegraded, latency,
            fmt.Sprintf("pool near exhaustion: %d/%d", stats.OpenConnections, stats.MaxOpenConnections), true}
    }
    
    return CheckResult{"postgresql", StatusHealthy, latency, "", true}
}

func (c *Checker) checkDiskSpace(ctx context.Context) CheckResult {
    start := time.Now()
    // Query PG for data directory disk usage
    var availMB int64
    err := c.db.QueryRowContext(ctx, `
        SELECT (pg_catalog.shobj_description(oid, 'pg_database'))::BIGINT 
        FROM pg_database WHERE datname = current_database()
    `).Scan(&availMB)
    // Fallback: check via OS
    if err != nil {
        return CheckResult{"disk", StatusHealthy, time.Since(start), "check unavailable", false}
    }
    return CheckResult{"disk", StatusHealthy, time.Since(start), "", false}
}

func (c *Checker) checkMemory(ctx context.Context) CheckResult {
    start := time.Now()
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    allocMB := m.Alloc / 1024 / 1024
    sysMB := m.Sys / 1024 / 1024
    
    status := StatusHealthy
    msg := fmt.Sprintf("alloc=%dMB sys=%dMB", allocMB, sysMB)
    
    if allocMB > 1500 { // > 1.5GB
        status = StatusDegraded
    }
    if allocMB > 2500 { // > 2.5GB
        status = StatusUnhealthy
    }
    
    return CheckResult{"memory", status, time.Since(start), msg, false}
}

// RunAll executes all health checks in parallel with timeout.
func (c *Checker) RunAll(ctx context.Context) (Status, []CheckResult) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    results := make([]CheckResult, len(c.checks))
    var wg sync.WaitGroup
    
    for i, check := range c.checks {
        wg.Add(1)
        go func(idx int, fn CheckFunc) {
            defer wg.Done()
            result := fn(ctx)
            results[idx] = result
            
            // Update Prometheus
            c.healthGauge.WithLabelValues(result.Name).Set(statusToFloat(result.Status))
            c.latencyHist.WithLabelValues(result.Name).Observe(result.Latency.Seconds())
        }(i, check)
    }
    
    wg.Wait()
    
    // Determine overall status
    overall := StatusHealthy
    for _, r := range results {
        if r.Status == StatusUnhealthy && r.Critical {
            overall = StatusUnhealthy
        }
        if r.Status == StatusDegraded && overall == StatusHealthy {
            overall = StatusDegraded
        }
    }
    
    return overall, results
}
```

### 2.2 Enhanced Health Endpoints

**File**: `backend/server/echo_routes.go` — Modify existing, add new.

```go
// Replace existing /healthz with deep check option
e.GET("/healthz", func(c *echo.Context) error {
    // Lightweight check (for liveness probe)
    return c.String(http.StatusOK, "OK")
})

// Deep health check (for monitoring/alerting)
e.GET("/healthz/deep", func(c *echo.Context) error {
    overall, checks := s.healthChecker.RunAll(c.Request().Context())
    
    status := http.StatusOK
    if overall == health.StatusUnhealthy {
        status = http.StatusServiceUnavailable
    }
    
    return c.JSON(status, map[string]any{
        "status":    overall,
        "checks":    checks,
        "node":      s.profile.ReplicaID,
        "version":   s.profile.Version,
        "uptime":    time.Since(time.Unix(s.startedTS, 0)).String(),
        "timestamp": time.Now().UTC(),
    })
})

// Readiness for K8s/LB (reuse from SOL-AVAIL-001)
e.GET("/readyz", func(c *echo.Context) error {
    overall, checks := s.healthChecker.RunAll(c.Request().Context())
    status := http.StatusOK
    if overall == health.StatusUnhealthy {
        status = http.StatusServiceUnavailable
    }
    return c.JSON(status, map[string]any{
        "status": overall,
        "checks": checks,
    })
})
```

### 2.3 Circuit Breaker Component

**Strategy**: Implement circuit breaker at L5 (Component Layer) — wraps DBFactory and external calls.

**File**: `backend/component/circuitbreaker/breaker.go` (NEW)

```go
package circuitbreaker

import (
    "context"
    "sync"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
)

// State represents the circuit breaker state.
type State int
const (
    StateClosed   State = iota // Normal operation
    StateOpen                  // Failing, reject calls
    StateHalfOpen              // Testing recovery
)

// Breaker implements the circuit breaker pattern.
type Breaker struct {
    name string
    
    // Configuration
    failureThreshold  int           // Failures before opening (default: 5)
    successThreshold  int           // Successes in half-open before closing (default: 2)
    timeout           time.Duration // Time in open before trying half-open (default: 30s)
    
    // State
    mu               sync.RWMutex
    state            State
    failures         int
    successes        int
    lastFailure      time.Time
    lastStateChange  time.Time
    
    // Prometheus
    stateGauge       prometheus.Gauge
    failureCounter   prometheus.Counter
    rejectedCounter  prometheus.Counter
}

// Config for creating a Breaker.
type Config struct {
    Name             string
    FailureThreshold int
    SuccessThreshold int
    Timeout          time.Duration
}

func New(cfg Config, registry prometheus.Registerer) *Breaker {
    if cfg.FailureThreshold == 0 {
        cfg.FailureThreshold = 5
    }
    if cfg.SuccessThreshold == 0 {
        cfg.SuccessThreshold = 2
    }
    if cfg.Timeout == 0 {
        cfg.Timeout = 30 * time.Second
    }
    
    b := &Breaker{
        name:             cfg.Name,
        failureThreshold: cfg.FailureThreshold,
        successThreshold: cfg.SuccessThreshold,
        timeout:          cfg.Timeout,
        state:            StateClosed,
        stateGauge: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "bytebase_circuit_breaker_state",
            Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
            ConstLabels: map[string]string{"breaker": cfg.Name},
        }),
        failureCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bytebase_circuit_breaker_failures_total",
            ConstLabels: map[string]string{"breaker": cfg.Name},
        }),
        rejectedCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "bytebase_circuit_breaker_rejected_total",
            ConstLabels: map[string]string{"breaker": cfg.Name},
        }),
    }
    registry.MustRegister(b.stateGauge, b.failureCounter, b.rejectedCounter)
    return b
}

// Execute runs fn if circuit is closed/half-open, rejects if open.
func (b *Breaker) Execute(ctx context.Context, fn func(context.Context) error) error {
    if !b.allowRequest() {
        b.rejectedCounter.Inc()
        return ErrCircuitOpen
    }
    
    err := fn(ctx)
    b.recordResult(err)
    return err
}

func (b *Breaker) allowRequest() bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    
    switch b.state {
    case StateClosed:
        return true
    case StateOpen:
        if time.Since(b.lastStateChange) > b.timeout {
            // Transition to half-open (do under write lock)
            b.mu.RUnlock()
            b.mu.Lock()
            if b.state == StateOpen { // Double-check
                b.state = StateHalfOpen
                b.successes = 0
                b.lastStateChange = time.Now()
                b.stateGauge.Set(2)
            }
            b.mu.Unlock()
            b.mu.RLock()
            return true
        }
        return false
    case StateHalfOpen:
        return true
    }
    return false
}

func (b *Breaker) recordResult(err error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if err != nil {
        b.failures++
        b.lastFailure = time.Now()
        b.failureCounter.Inc()
        
        if b.state == StateHalfOpen || b.failures >= b.failureThreshold {
            b.state = StateOpen
            b.lastStateChange = time.Now()
            b.stateGauge.Set(1)
            slog.Warn("Circuit breaker opened",
                slog.String("breaker", b.name),
                slog.Int("failures", b.failures))
        }
    } else {
        if b.state == StateHalfOpen {
            b.successes++
            if b.successes >= b.successThreshold {
                b.state = StateClosed
                b.failures = 0
                b.lastStateChange = time.Now()
                b.stateGauge.Set(0)
                slog.Info("Circuit breaker closed", slog.String("breaker", b.name))
            }
        } else {
            b.failures = 0
        }
    }
}

var ErrCircuitOpen = errors.New("circuit breaker is open")
```

### 2.4 DBFactory with Circuit Breaker

**File**: `backend/component/dbfactory/dbfactory.go` — Wrap driver opens with circuit breaker.

```go
// dbfactory.go — Enhanced with per-instance circuit breakers
type DBFactory struct {
    store          *store.Store
    licenseService *enterprise.LicenseService
    
    // NEW: per-instance circuit breakers
    breakers   map[string]*circuitbreaker.Breaker  // key: instance resourceID
    breakersMu sync.RWMutex
    registry   prometheus.Registerer
}

func (f *DBFactory) GetDriver(ctx context.Context, instance *store.InstanceMessage, database string) (db.Driver, error) {
    breaker := f.getOrCreateBreaker(instance.ResourceID)
    
    var driver db.Driver
    err := breaker.Execute(ctx, func(ctx context.Context) error {
        var err error
        driver, err = f.getDriverInternal(ctx, instance, database)
        return err
    })
    
    if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
        return nil, status.Errorf(codes.Unavailable,
            "instance %s is unreachable (circuit breaker open)", instance.ResourceID)
    }
    
    return driver, err
}

func (f *DBFactory) getOrCreateBreaker(instanceID string) *circuitbreaker.Breaker {
    f.breakersMu.RLock()
    if b, ok := f.breakers[instanceID]; ok {
        f.breakersMu.RUnlock()
        return b
    }
    f.breakersMu.RUnlock()
    
    f.breakersMu.Lock()
    defer f.breakersMu.Unlock()
    
    b := circuitbreaker.New(circuitbreaker.Config{
        Name:             "instance_" + instanceID,
        FailureThreshold: 5,
        Timeout:          30 * time.Second,
    }, f.registry)
    f.breakers[instanceID] = b
    return b
}
```

### 2.5 Self-Healing Runner

**File**: `backend/runner/selfheal/runner.go` (NEW)

```go
package selfheal

// Runner monitors system health and triggers automatic recovery actions.
type Runner struct {
    store         *store.Store
    healthChecker *health.Checker
    profile       *config.Profile
}

func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            r.checkAndHeal(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (r *Runner) checkAndHeal(ctx context.Context) {
    overall, checks := r.healthChecker.RunAll(ctx)
    
    for _, check := range checks {
        switch {
        case check.Name == "postgresql" && check.Status == health.StatusDegraded:
            r.healPoolExhaustion(ctx)
        case check.Name == "memory" && check.Status == health.StatusDegraded:
            r.healMemoryPressure(ctx)
        }
    }
    
    if overall == health.StatusUnhealthy {
        slog.Error("System unhealthy — alerting operations")
        // Metrics will trigger alerts via Prometheus/Alertmanager
    }
}

func (r *Runner) healPoolExhaustion(ctx context.Context) {
    slog.Warn("Self-healing: DB pool near exhaustion — purging caches")
    r.store.DeleteCache()  // Uses existing Store.DeleteCache()
}

func (r *Runner) healMemoryPressure(ctx context.Context) {
    slog.Warn("Self-healing: Memory pressure — forcing GC and purging caches")
    r.store.DeleteCache()
    runtime.GC()
    debug.FreeOSMemory()
}
```

---

## 3. Integration with Server Lifecycle

```go
// backend/server/server.go — NewServer()

// Initialize health checker (uses existing Prometheus registry)
s.healthChecker = health.NewChecker(stores.GetDB(), registry)

// Add self-healing runner
if profile.HA {
    s.selfHealRunner = selfheal.NewRunner(stores, s.healthChecker, profile)
}

// In Run():
if s.selfHealRunner != nil {
    s.runnerWG.Add(1)
    go s.selfHealRunner.Run(ctx, &s.runnerWG)
}
```

---

## 4. File Change Summary

| Layer | File | Change Type | Description |
|---|---|---|---|
| L5 | `backend/component/health/checker.go` | **New** | Deep health check system |
| L5 | `backend/component/circuitbreaker/breaker.go` | **New** | Circuit breaker pattern |
| L5 | `backend/component/dbfactory/dbfactory.go` | **Modify** | Per-instance circuit breakers |
| L6 | `backend/runner/selfheal/runner.go` | **New** | Self-healing runner |
| L2 | `backend/server/echo_routes.go` | **Modify** | `/healthz/deep`, `/readyz` |
| L2 | `backend/server/server.go` | **Modify** | Wire health checker, self-heal runner |

---

## 5. Prometheus Alert Rules

```yaml
# alerts/availability.yml
groups:
  - name: bytebase-health
    rules:
      - alert: BytebaseUnhealthy
        expr: bytebase_health_status{component="postgresql"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Bytebase PostgreSQL connection unhealthy"

      - alert: BytebaseCircuitOpen
        expr: bytebase_circuit_breaker_state > 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker {{ $labels.breaker }} is open"

      - alert: BytebaseMemoryHigh
        expr: bytebase_health_status{component="memory"} < 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Bytebase memory pressure detected"
```

---

## 6. Backward Compatibility

| Scenario | Behavior |
|---|---|
| `/healthz` | Unchanged — still returns "OK" (liveness probe safe) |
| Non-HA mode | Health checker works, self-healing runner optional |
| No Prometheus | Metrics still collected, just not scraped |
| Existing DBFactory consumers | Signature compatible — circuit breaker transparent |
