# Solution: Resilience Patterns Infrastructure — CR-ARCH-009

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-009                                             |
| **CR Reference**   | CR-ARCH-009                                              |
| **Title**          | Circuit Breaker + Bulkhead + Rate Limiter + Retry Library |
| **Affected Layers**| L4 (Service), L5 (Component), L6 (Runner)                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §6 (L6 — Runner Layer):
- 8 background runners: TaskScheduler, SchemaSyncer, ApprovalRunner, PlanCheckScheduler, NotifyListener, DataCleaner, HeartbeatRunner, MemoryMonitor
- No circuit breakers, no bulkheads, no structured retry

Per [TDD.md](../../TDD.md) §6:
- Runners use `time.Sleep(100ms)` for DB reconnection (no backoff)
- Schema sync processes ALL instances concurrently (no concurrency limit)

---

## 2. Solution Design

### 2.1 Circuit Breaker

**New file**: `backend/common/resilience/circuit_breaker.go`

```go
package resilience

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
)

// State represents the circuit breaker state.
type State int

const (
    StateClosed   State = iota  // Normal — requests pass through
    StateOpen                   // Tripped — requests fail immediately
    StateHalfOpen               // Recovery — allow one test request
)

func (s State) String() string {
    switch s {
    case StateClosed:  return "closed"
    case StateOpen:    return "open"
    case StateHalfOpen: return "half_open"
    default: return "unknown"
    }
}

// CircuitBreaker prevents cascading failures by stopping requests
// to a failing dependency after threshold failures.
type CircuitBreaker struct {
    mu           sync.Mutex
    name         string
    state        State
    failures     int
    maxFailures  int
    resetTimeout time.Duration
    lastFailure  time.Time
    metrics      *cbMetrics
}

// CircuitBreakerConfig configures a circuit breaker instance.
type CircuitBreakerConfig struct {
    Name         string
    MaxFailures  int           // Failures before circuit opens (default: 5)
    ResetTimeout time.Duration // Time in Open before trying HalfOpen (default: 30s)
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
    if cfg.MaxFailures == 0 { cfg.MaxFailures = 5 }
    if cfg.ResetTimeout == 0 { cfg.ResetTimeout = 30 * time.Second }

    cb := &CircuitBreaker{
        name:         cfg.Name,
        state:        StateClosed,
        maxFailures:  cfg.MaxFailures,
        resetTimeout: cfg.ResetTimeout,
        metrics:      newCBMetrics(cfg.Name),
    }
    return cb
}

// Execute runs the function through the circuit breaker.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
    cb.mu.Lock()
    state := cb.currentState()
    cb.mu.Unlock()

    switch state {
    case StateOpen:
        cb.metrics.rejected.Inc()
        return fmt.Errorf("circuit breaker [%s] is open", cb.name)

    case StateHalfOpen:
        // Allow one probe request
        err := fn(ctx)
        cb.mu.Lock()
        defer cb.mu.Unlock()
        if err != nil {
            cb.toOpen()
            return err
        }
        cb.toClosed()
        return nil

    case StateClosed:
        err := fn(ctx)
        if err != nil {
            cb.mu.Lock()
            cb.failures++
            cb.lastFailure = time.Now()
            if cb.failures >= cb.maxFailures {
                cb.toOpen()
            }
            cb.mu.Unlock()
            cb.metrics.failures.Inc()
            return err
        }
        cb.mu.Lock()
        cb.failures = 0
        cb.mu.Unlock()
        cb.metrics.successes.Inc()
        return nil
    }
    return nil
}

func (cb *CircuitBreaker) currentState() State {
    if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
        cb.state = StateHalfOpen
        cb.metrics.stateGauge.Set(float64(StateHalfOpen))
    }
    return cb.state
}

func (cb *CircuitBreaker) toOpen() {
    cb.state = StateOpen
    cb.metrics.stateGauge.Set(float64(StateOpen))
}

func (cb *CircuitBreaker) toClosed() {
    cb.state = StateClosed
    cb.failures = 0
    cb.metrics.stateGauge.Set(float64(StateClosed))
}

type cbMetrics struct {
    stateGauge *prometheus.Gauge
    failures   prometheus.Counter
    successes  prometheus.Counter
    rejected   prometheus.Counter
}

func newCBMetrics(name string) *cbMetrics {
    labels := prometheus.Labels{"name": name}
    sg := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_circuit_breaker_state",
        Help: "Circuit breaker state (0=closed, 1=open, 2=half_open)",
        ConstLabels: labels,
    })
    f := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_circuit_breaker_failures_total",
        ConstLabels: labels,
    })
    s := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_circuit_breaker_successes_total",
        ConstLabels: labels,
    })
    r := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "bytebase_circuit_breaker_rejected_total",
        ConstLabels: labels,
    })
    prometheus.MustRegister(sg, f, s, r)
    return &cbMetrics{stateGauge: &sg, failures: f, successes: s, rejected: r}
}
```

### 2.2 Bulkhead (Concurrency Limiter)

**New file**: `backend/common/resilience/bulkhead.go`

```go
package resilience

import (
    "context"
    "fmt"

    "github.com/prometheus/client_golang/prometheus"
)

// Bulkhead limits concurrency for resource-intensive operations.
type Bulkhead struct {
    name    string
    sem     chan struct{}
    metrics *bulkheadMetrics
}

func NewBulkhead(name string, maxConcurrent int) *Bulkhead {
    return &Bulkhead{
        name:    name,
        sem:     make(chan struct{}, maxConcurrent),
        metrics: newBulkheadMetrics(name, maxConcurrent),
    }
}

// Execute runs fn within the concurrency limit.
// Blocks if at capacity. Respects context cancellation.
func (b *Bulkhead) Execute(ctx context.Context, fn func(context.Context) error) error {
    b.metrics.queued.Inc()
    defer b.metrics.queued.Dec()

    select {
    case b.sem <- struct{}{}:
        // Acquired permit
        defer func() { <-b.sem }()
        b.metrics.active.Inc()
        defer b.metrics.active.Dec()
        return fn(ctx)
    case <-ctx.Done():
        return fmt.Errorf("bulkhead [%s]: %w", b.name, ctx.Err())
    }
}

type bulkheadMetrics struct {
    active *prometheus.Gauge
    queued *prometheus.Gauge
}

func newBulkheadMetrics(name string, max int) *bulkheadMetrics {
    labels := prometheus.Labels{"name": name}
    a := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_bulkhead_active",
        ConstLabels: labels,
    })
    q := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_bulkhead_queued",
        ConstLabels: labels,
    })
    m := prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_bulkhead_max",
        ConstLabels: labels,
    })
    prometheus.MustRegister(a, q, m)
    m.Set(float64(max))
    return &bulkheadMetrics{active: &a, queued: &q}
}
```

### 2.3 Retry with Exponential Backoff

**New file**: `backend/common/resilience/retry.go`

```go
package resilience

import (
    "context"
    "math"
    "math/rand"
    "time"

    "github.com/prometheus/client_golang/prometheus"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
    MaxRetries   int           // Max number of retries (default: 3)
    InitialDelay time.Duration // First delay (default: 100ms)
    MaxDelay     time.Duration // Cap on delay (default: 30s)
    Multiplier   float64       // Backoff multiplier (default: 2.0)
    Jitter       bool          // Add random jitter (default: true)
}

var DefaultRetryConfig = RetryConfig{
    MaxRetries:   3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       true,
}

// Retry executes fn with exponential backoff.
func Retry(ctx context.Context, name string, cfg RetryConfig, fn func(context.Context) error) error {
    if cfg.MaxRetries == 0 { cfg = DefaultRetryConfig }

    var lastErr error
    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        if attempt > 0 {
            delay := calculateDelay(attempt, cfg)
            retryCounter.WithLabelValues(name, fmt.Sprintf("%d", attempt)).Inc()

            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        lastErr = fn(ctx)
        if lastErr == nil {
            return nil
        }
    }
    return fmt.Errorf("retry [%s] exhausted after %d attempts: %w", name, cfg.MaxRetries, lastErr)
}

func calculateDelay(attempt int, cfg RetryConfig) time.Duration {
    delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
    if delay > float64(cfg.MaxDelay) {
        delay = float64(cfg.MaxDelay)
    }
    if cfg.Jitter {
        delay = delay * (0.5 + rand.Float64()*0.5)  // 50-100% of calculated delay
    }
    return time.Duration(delay)
}

var retryCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "bytebase_retry_total",
    Help: "Total retry attempts per operation",
}, []string{"operation", "attempt"})

func init() {
    prometheus.MustRegister(retryCounter)
}
```

### 2.4 API Rate Limiter

**New file**: `backend/common/resilience/rate_limiter.go`

```go
package resilience

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// RateLimiter implements per-key token bucket rate limiting.
type RateLimiter struct {
    mu      sync.Mutex
    buckets map[string]*tokenBucket
    rate    float64       // tokens per second
    burst   int           // max tokens
    cleanup time.Duration // cleanup interval for stale buckets
}

type tokenBucket struct {
    tokens    float64
    lastFill  time.Time
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
    rl := &RateLimiter{
        buckets: make(map[string]*tokenBucket),
        rate:    rate,
        burst:   burst,
        cleanup: 5 * time.Minute,
    }
    go rl.cleanupLoop()
    return rl
}

// Allow checks if a request is allowed for the given key.
func (rl *RateLimiter) Allow(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    bucket, ok := rl.buckets[key]
    if !ok {
        bucket = &tokenBucket{
            tokens:   float64(rl.burst),
            lastFill: time.Now(),
        }
        rl.buckets[key] = bucket
    }

    // Refill tokens based on elapsed time
    elapsed := time.Since(bucket.lastFill).Seconds()
    bucket.tokens += elapsed * rl.rate
    if bucket.tokens > float64(rl.burst) {
        bucket.tokens = float64(rl.burst)
    }
    bucket.lastFill = time.Now()

    // Check if request is allowed
    if bucket.tokens >= 1 {
        bucket.tokens--
        return true
    }
    return false
}

func (rl *RateLimiter) cleanupLoop() {
    ticker := time.NewTicker(rl.cleanup)
    defer ticker.Stop()
    for range ticker.C {
        rl.mu.Lock()
        for key, bucket := range rl.buckets {
            if time.Since(bucket.lastFill) > rl.cleanup {
                delete(rl.buckets, key)
            }
        }
        rl.mu.Unlock()
    }
}
```

### 2.5 Application — Webhook Circuit Breaker

**Modified file**: `backend/component/webhook/manager.go`

```go
type Manager struct {
    store          *store.Store
    profile        *config.Profile
    circuitBreaker *resilience.CircuitBreaker  // NEW
}

func NewManager(store *store.Store, profile *config.Profile) *Manager {
    return &Manager{
        store:   store,
        profile: profile,
        circuitBreaker: resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
            Name:         "webhook",
            MaxFailures:  5,
            ResetTimeout: 30 * time.Second,
        }),
    }
}

func (m *Manager) Send(ctx context.Context, webhook *WebhookMessage) error {
    // BEFORE: direct HTTP call, no protection
    // AFTER: wrapped in circuit breaker
    return m.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
        return m.doSend(ctx, webhook)
    })
}
```

### 2.6 Application — Schema Sync Bulkhead

**Modified file**: `backend/runner/schemasync/syncer.go`

```go
type Syncer struct {
    stores  *store.Store
    bulkhead *resilience.Bulkhead  // NEW: limit to 10 concurrent syncs
}

func NewSyncer(stores *store.Store, ...) *Syncer {
    return &Syncer{
        stores:   stores,
        bulkhead: resilience.NewBulkhead("schema_sync", 10),
    }
}

func (s *Syncer) syncAllInstances(ctx context.Context, instances []*store.InstanceMessage) {
    var wg sync.WaitGroup
    for _, inst := range instances {
        wg.Add(1)
        go func(instance *store.InstanceMessage) {
            defer wg.Done()
            // BEFORE: unlimited concurrency
            // AFTER: max 10 concurrent via bulkhead
            if err := s.bulkhead.Execute(ctx, func(ctx context.Context) error {
                return s.syncInstance(ctx, instance)
            }); err != nil {
                slog.Warn("Schema sync failed", "instance", instance.UID, "error", err)
            }
        }(inst)
    }
    wg.Wait()
}
```

### 2.7 Application — DB Reconnection Retry

**Modified file**: `backend/store/db_connection.go`

```go
func (m *DBConnectionManager) reloadConnection(ctx context.Context, filePath string) {
    // BEFORE: time.Sleep(100ms) — fixed delay
    // AFTER: exponential backoff retry
    err := resilience.Retry(ctx, "db_reconnect", resilience.RetryConfig{
        MaxRetries:   5,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
        Jitter:       true,
    }, func(ctx context.Context) error {
        newURL, err := readURLFromFile(filePath)
        if err != nil { return err }
        newDB, err := createConnectionWithTracer(ctx, newURL)
        if err != nil { return err }
        m.swapConnection(newDB)
        return nil
    })
    if err != nil {
        slog.Error("DB reconnection failed after retries", "error", err)
    }
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/common/resilience/circuit_breaker.go` | L5 | **NEW** | Circuit breaker |
| `backend/common/resilience/bulkhead.go` | L5 | **NEW** | Concurrency limiter |
| `backend/common/resilience/retry.go` | L5 | **NEW** | Exponential backoff |
| `backend/common/resilience/rate_limiter.go` | L5 | **NEW** | Token bucket rate limiter |
| `backend/component/webhook/manager.go` | L5 | **MODIFY** | Add circuit breaker |
| `backend/runner/schemasync/syncer.go` | L6 | **MODIFY** | Add bulkhead (10 concurrent) |
| `backend/store/db_connection.go` | L8 | **MODIFY** | Replace fixed delay with retry |

---

## 5. Dependency Direction Validation

```
L5 (webhook/manager.go)     → resilience (L5 common — same layer)
L6 (schemasync/syncer.go)   → resilience (L5 common — allowed: upper→lower)
L8 (db_connection.go)       → resilience (L5 common — reverse direction!)
```

**Note**: L8 → L5 breaks strict layering. **Mitigation**: Move retry logic to L5 component wrapper or accept this pragmatic exception since `resilience` is a utility package.

---

## 6. Rollback Plan

Each pattern is independently removable:
1. Circuit breaker: remove from webhook manager → direct calls
2. Bulkhead: remove from syncer → unlimited concurrency
3. Retry: replace with `time.Sleep(100ms)` → fixed delay
