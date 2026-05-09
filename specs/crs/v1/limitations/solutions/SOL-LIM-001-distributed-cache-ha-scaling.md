# Solution: CR-LIM-001 — Distributed Cache & HA Horizontal Scaling

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-LIM-001                               |
| **Solution ID**| SOL-LIM-001                              |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-09                               |
| **Arch Refs**  | L5 (Component — Bus), L6 (Runner), L8 (Store), L10 (Infrastructure) |
| **TDD Refs**   | §4 Data Access Layer, §5 Background Runner System, §14 Trade-offs |

---

## 1. Solution Overview

### 1.1 Approach Summary

3-phase approach tối ưu hóa cho **minimal external dependency** và **incremental rollout**:

1. **Phase A — Cache Interface + Redis Adapter** (core scalability win)
2. **Phase B — Runner Leader Election via PG Advisory Locks** (zero new deps)
3. **Phase C — Read Replica Routing** (optional, for extreme scale)

### 1.2 Design Rationale

Từ TDD §4.1, Store hiện sử dụng `hashicorp/golang-lru` với capacity 1K-32K entries. HA mode disables cache hoàn toàn (`enableCache=false` — TDD §4.2). Giải pháp cần **giữ nguyên cache semantics** khi chuyển sang distributed backend.

Từ Architecture L6, 8 runners chạy dưới dạng goroutines (`server.go:Run()`). HA mode không có coordination → duplicate work. Advisory locks (`backend/store/advisory_lock.go`) đã tồn tại trong codebase — tận dụng pattern này cho leader election.

Từ Architecture L8, Store là "most depended-upon layer" (dependency matrix). Thay đổi ở L8 phải **transparent** với L3-L7 — interface không đổi.

---

## 2. Detailed Technical Design

### 2.1 Phase A — Cache Interface Abstraction + Redis Adapter

#### 2.1.1 Cache Interface

**File**: `backend/store/cache.go` (new)

```go
// Cache provides a generic key-value cache with TTL support.
// Implementations: LRUCache (single-node), RedisCache (HA mode).
type Cache[K comparable, V any] interface {
    // Get retrieves a value. Returns (value, true) if found, (zero, false) if miss.
    Get(ctx context.Context, key K) (V, bool)
    // Set stores a value with optional TTL. TTL=0 means no expiry.
    Set(ctx context.Context, key K, value V, ttl time.Duration) error
    // Delete removes a key.
    Delete(ctx context.Context, key K) error
    // Purge removes all entries matching a prefix pattern.
    Purge(ctx context.Context, prefix string) error
    // Stats returns cache statistics.
    Stats() CacheStats
}

type CacheStats struct {
    Hits       int64
    Misses     int64
    Evictions  int64
    Size       int64
}
```

**Rationale**: Generic interface cho phép swap backend mà không thay đổi caller code. `context.Context` cho phép timeout/cancellation ở Redis adapter.

#### 2.1.2 LRU Cache Adapter (Single-Node)

**File**: `backend/store/cache_lru.go` (new — wraps existing `hashicorp/golang-lru`)

```go
// lruCache wraps hashicorp/golang-lru to implement Cache[K,V].
// Used in single-node mode (non-HA). Zero external dependency.
type lruCache[K comparable, V any] struct {
    inner *lru.Cache[K, V]
    mu    sync.RWMutex
    stats cacheStatsCollector
}

func NewLRUCache[K comparable, V any](size int) Cache[K, V] {
    c, _ := lru.New[K, V](size)
    return &lruCache[K, V]{inner: c}
}

func (c *lruCache[K, V]) Get(_ context.Context, key K) (V, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.inner.Get(key)
    if ok {
        c.stats.hits.Add(1)
    } else {
        c.stats.misses.Add(1)
    }
    return v, ok
}

// Set, Delete, Purge, Stats — straightforward LRU delegation
```

**Key**: `context.Context` ignored in LRU mode — parameter exists for interface compatibility.

#### 2.1.3 Redis Cache Adapter (HA Mode)

**File**: `backend/store/cache_redis.go` (new)

```go
import "github.com/redis/go-redis/v9"

// redisCache implements Cache[K,V] using Redis as backing store.
// Used in HA mode where multiple replicas need cache coherence.
type redisCache[K comparable, V any] struct {
    client     redis.UniversalClient
    prefix     string           // namespace prefix: "bb:user:", "bb:inst:", etc.
    defaultTTL time.Duration
    marshal    func(V) ([]byte, error)
    unmarshal  func([]byte) (V, error)
    stats      cacheStatsCollector
}

func NewRedisCache[K comparable, V any](
    client redis.UniversalClient,
    prefix string,
    defaultTTL time.Duration,
    marshal func(V) ([]byte, error),
    unmarshal func([]byte) (V, error),
) Cache[K, V] {
    return &redisCache[K, V]{
        client:     client,
        prefix:     prefix,
        defaultTTL: defaultTTL,
        marshal:    marshal,
        unmarshal:  unmarshal,
    }
}

func (c *redisCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
    data, err := c.client.Get(ctx, c.keyString(key)).Bytes()
    if err == redis.Nil {
        c.stats.misses.Add(1)
        var zero V
        return zero, false
    }
    if err != nil {
        // Redis error → treat as miss (graceful degradation)
        slog.Warn("redis cache get error", "key", key, "err", err)
        c.stats.misses.Add(1)
        var zero V
        return zero, false
    }
    v, err := c.unmarshal(data)
    if err != nil {
        c.stats.misses.Add(1)
        var zero V
        return zero, false
    }
    c.stats.hits.Add(1)
    return v, true
}

func (c *redisCache[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
    data, err := c.marshal(value)
    if err != nil {
        return err
    }
    if ttl == 0 {
        ttl = c.defaultTTL
    }
    return c.client.Set(ctx, c.keyString(key), data, ttl).Err()
}

func (c *redisCache[K, V]) Delete(ctx context.Context, key K) error {
    return c.client.Del(ctx, c.keyString(key)).Err()
}

func (c *redisCache[K, V]) Purge(ctx context.Context, prefix string) error {
    // Use SCAN + DEL to avoid blocking KEYS command
    iter := c.client.Scan(ctx, 0, c.prefix+prefix+"*", 100).Iterator()
    var keys []string
    for iter.Next(ctx) {
        keys = append(keys, iter.Val())
    }
    if len(keys) > 0 {
        return c.client.Del(ctx, keys...).Err()
    }
    return nil
}

func (c *redisCache[K, V]) keyString(key K) string {
    return fmt.Sprintf("%s%v", c.prefix, key)
}
```

**Serialization**: Use `protojson.Marshal/Unmarshal` for proto messages (consistent with TDD §4.4 JSONB pattern). For non-proto types, use `encoding/json`.

#### 2.1.4 Store Modification — Cache Factory

**File**: `backend/store/store.go` (modify `New()`)

```go
func New(ctx context.Context, pgURL string, enableCache bool, redisURL string) (*Store, error) {
    s := &Store{
        dbConnManager: newDBConnectionManager(pgURL),
    }

    if enableCache {
        if redisURL != "" {
            // HA mode with Redis — distributed cache
            client := redis.NewUniversalClient(&redis.UniversalOptions{
                Addrs: strings.Split(redisURL, ","),
            })
            if err := client.Ping(ctx).Err(); err != nil {
                slog.Warn("Redis unavailable, falling back to no-cache", "err", err)
                s.initNullCaches()
                return s, nil
            }
            s.redisClient = client
            s.initRedisCaches(client)
        } else {
            // Single-node — LRU cache (existing behavior)
            s.initLRUCaches()
        }
    } else {
        // Cache disabled (HA mode without Redis — current behavior)
        s.initNullCaches()
    }

    return s, nil
}
```

**Key change**: `server.go` hiện gọi `store.New(ctx, pgURL, !profile.HA)`. Sau thay đổi:

```go
// backend/server/server.go (modified)
enableCache := true  // Always enable cache
redisURL := os.Getenv("REDIS_URL")
if profile.HA && redisURL == "" {
    enableCache = false  // HA without Redis → no cache (backward compat)
}
storeInstance, err := store.New(ctx, pgURL, enableCache, redisURL)
```

#### 2.1.5 Cache Invalidation via PG NOTIFY

**File**: `backend/store/cache_invalidator.go` (new)

```go
// CacheInvalidator listens to PostgreSQL NOTIFY events and invalidates
// corresponding cache entries. Works with both LRU and Redis caches.
type CacheInvalidator struct {
    store      *Store
    pgPool     *pgxpool.Pool
    cancelFunc context.CancelFunc
}

func (inv *CacheInvalidator) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    conn, err := inv.pgPool.Acquire(ctx)
    if err != nil {
        return
    }
    defer conn.Release()

    // Listen on cache invalidation channel
    _, _ = conn.Exec(ctx, "LISTEN cache_invalidation")

    for {
        notification, err := conn.Conn().WaitForNotification(ctx)
        if err != nil {
            if ctx.Err() != nil {
                return // Shutdown
            }
            time.Sleep(time.Second) // Retry
            continue
        }
        inv.handleNotification(ctx, notification.Payload)
    }
}

func (inv *CacheInvalidator) handleNotification(ctx context.Context, payload string) {
    var event struct {
        Table  string `json:"table"`
        Action string `json:"action"`
        ID     string `json:"id"`
    }
    if err := json.Unmarshal([]byte(payload), &event); err != nil {
        return
    }

    // Route invalidation to appropriate cache
    switch event.Table {
    case "principal":
        inv.store.userEmailCache.Delete(ctx, event.ID)
    case "instance":
        inv.store.instanceCache.Delete(ctx, event.ID)
    case "db":
        inv.store.databaseCache.Delete(ctx, event.ID)
    case "project":
        inv.store.projectCache.Delete(ctx, event.ID)
    case "policy":
        inv.store.policyCache.Purge(ctx, event.ID)
    case "setting":
        inv.store.settingCache.Delete(ctx, event.ID)
    }
}
```

**Migration**: PG NOTIFY triggers (see CR-LIM-001 §3.3 for SQL).

### 2.2 Phase B — Runner Leader Election via PG Advisory Locks

#### 2.2.1 Leader Election Engine

**File**: `backend/component/leader/election.go` (new)

```go
// LeaderElector uses PostgreSQL advisory locks to elect a leader
// among replicas. No external dependency (etcd/consul) needed.
// Leverages existing advisory_lock.go patterns from L8.
type LeaderElector struct {
    db         *sql.DB
    lockID     int64       // Unique per runner type
    isLeader   atomic.Bool
    renewTick  time.Duration
    onAcquired func()
    onLost     func()
}

// Advisory lock ID allocation (deterministic, collision-free)
const (
    LockIDTaskScheduler   int64 = 100001
    LockIDPlanCheck       int64 = 100002
    LockIDSchemaSync      int64 = 100003
    LockIDApproval        int64 = 100004
    LockIDDataCleaner     int64 = 100005
    LockIDNotifyListener  int64 = 100006
    LockIDHeartbeat       int64 = 100007
    LockIDMemoryMonitor   int64 = 100008
)

func NewLeaderElector(db *sql.DB, lockID int64, renewTick time.Duration) *LeaderElector {
    return &LeaderElector{
        db:        db,
        lockID:    lockID,
        renewTick: renewTick,
    }
}

func (e *LeaderElector) Run(ctx context.Context) {
    ticker := time.NewTicker(e.renewTick)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            e.release()
            return
        case <-ticker.C:
            e.tryAcquire(ctx)
        }
    }
}

func (e *LeaderElector) tryAcquire(ctx context.Context) {
    // pg_try_advisory_lock returns true if lock acquired, false otherwise.
    // Session-level lock: held until connection closes (crash-safe).
    var acquired bool
    err := e.db.QueryRowContext(ctx,
        "SELECT pg_try_advisory_lock($1)", e.lockID,
    ).Scan(&acquired)

    if err != nil || !acquired {
        if e.isLeader.CompareAndSwap(true, false) {
            slog.Info("Lost leadership", "lockID", e.lockID)
            if e.onLost != nil {
                e.onLost()
            }
        }
        return
    }

    if e.isLeader.CompareAndSwap(false, true) {
        slog.Info("Acquired leadership", "lockID", e.lockID)
        if e.onAcquired != nil {
            e.onAcquired()
        }
    }
}

func (e *LeaderElector) release() {
    _, _ = e.db.Exec("SELECT pg_advisory_unlock($1)", e.lockID)
    e.isLeader.Store(false)
}

func (e *LeaderElector) IsLeader() bool {
    return e.isLeader.Load()
}
```

**Why PG advisory locks?**: 
- Already in codebase (`backend/store/advisory_lock.go`)
- No new dependency (etcd, consul, Redis)
- Session-level locks auto-release on crash (critical for failover)
- Same PostgreSQL instance used for metadata

#### 2.2.2 Leader-Guarded Runner Wrapper

**File**: `backend/runner/leader_runner.go` (new)

```go
// LeaderRunner wraps an existing runner, only executing it when this
// replica is the elected leader for the runner's lock ID.
type LeaderRunner struct {
    inner    Runner
    elector  *leader.LeaderElector
    name     string
}

type Runner interface {
    Run(ctx context.Context, wg *sync.WaitGroup)
}

func NewLeaderRunner(inner Runner, elector *leader.LeaderElector, name string) *LeaderRunner {
    return &LeaderRunner{inner: inner, elector: elector, name: name}
}

func (r *LeaderRunner) Run(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()

    // Start leader election in background
    go r.elector.Run(ctx)

    // Wait for leadership before starting runner
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if r.elector.IsLeader() {
                slog.Info("Starting runner as leader", "runner", r.name)
                wg.Add(1) // Re-add since we'll call inner.Run
                r.inner.Run(ctx, wg)
                return
            }
        }
    }
}
```

#### 2.2.3 Server Wiring

**File**: `backend/server/server.go` (modify `Run()`)

```go
func (s *Server) Run(ctx context.Context, port int) error {
    // ... existing setup ...

    if s.profile.HA {
        // HA mode: wrap runners with leader election
        db := s.store.GetDB()
        s.startLeaderRunner(ctx, s.taskScheduler, db, leader.LockIDTaskScheduler, "TaskScheduler")
        s.startLeaderRunner(ctx, s.schemaSyncer, db, leader.LockIDSchemaSync, "SchemaSync")
        s.startLeaderRunner(ctx, s.approvalRunner, db, leader.LockIDApproval, "Approval")
        s.startLeaderRunner(ctx, s.planCheckScheduler, db, leader.LockIDPlanCheck, "PlanCheck")
        // Non-exclusive runners (all replicas)
        go s.heartbeatRunner.Run(ctx, &s.runnerWG)    // All replicas report health
        go s.notifyListener.Run(ctx, &s.runnerWG)      // All replicas listen
    } else {
        // Single-node: direct goroutine launch (existing behavior)
        go s.taskScheduler.Run(ctx, &s.runnerWG)
        go s.schemaSyncer.Run(ctx, &s.runnerWG)
        go s.approvalRunner.Run(ctx, &s.runnerWG)
        go s.planCheckScheduler.Run(ctx, &s.runnerWG)
        go s.heartbeatRunner.Run(ctx, &s.runnerWG)
        go s.notifyListener.Run(ctx, &s.runnerWG)
    }
    // ...
}

func (s *Server) startLeaderRunner(ctx context.Context, r Runner, db *sql.DB, lockID int64, name string) {
    elector := leader.NewLeaderElector(db, lockID, 10*time.Second)
    wrapped := NewLeaderRunner(r, elector, name)
    s.runnerWG.Add(1)
    go wrapped.Run(ctx, &s.runnerWG)
}
```

### 2.3 Phase C — Read Replica Routing

#### 2.3.1 Dual Pool Manager

**File**: `backend/store/pool.go` (new)

```go
// PoolManager manages primary (read-write) and replica (read-only) connection pools.
type PoolManager struct {
    primary      *sql.DB
    replica      *sql.DB   // nil if no replica configured
    replicaLag   atomic.Int64  // microseconds
    lagThreshold time.Duration
}

func NewPoolManager(primaryURL, replicaURL string, lagThreshold time.Duration) (*PoolManager, error) {
    primary, err := createConnectionWithTracer(context.Background(), primaryURL)
    if err != nil {
        return nil, err
    }

    pm := &PoolManager{
        primary:      primary,
        lagThreshold: lagThreshold,
    }

    if replicaURL != "" {
        replica, err := createConnectionWithTracer(context.Background(), replicaURL)
        if err != nil {
            slog.Warn("Read replica unavailable, using primary only", "err", err)
        } else {
            pm.replica = replica
            go pm.monitorReplicaLag(context.Background())
        }
    }

    return pm, nil
}

// ForRead returns the best connection for read queries.
// Uses replica if available and lag is within threshold.
func (pm *PoolManager) ForRead() *sql.DB {
    if pm.replica == nil {
        return pm.primary
    }
    lag := time.Duration(pm.replicaLag.Load()) * time.Microsecond
    if lag > pm.lagThreshold {
        return pm.primary // Replica too far behind
    }
    return pm.replica
}

// ForWrite always returns the primary connection.
func (pm *PoolManager) ForWrite() *sql.DB {
    return pm.primary
}

func (pm *PoolManager) monitorReplicaLag(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            var lagMicros int64
            err := pm.replica.QueryRowContext(ctx,
                `SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp())) * 1000000`,
            ).Scan(&lagMicros)
            if err == nil {
                pm.replicaLag.Store(lagMicros)
            }
        }
    }
}
```

#### 2.3.2 Store Integration

**File**: `backend/store/store.go` — route read queries:

```go
// In list/get operations, use ForRead()
func (s *Store) ListDatabases(ctx context.Context, find *FindDatabaseMessage) ([]*DatabaseMessage, error) {
    db := s.poolManager.ForRead()  // Read replica if available
    // ... existing query logic using db ...
}

// In create/update/delete operations, use ForWrite()
func (s *Store) CreateDatabaseDefault(ctx context.Context, create *DatabaseMessage) (*DatabaseMessage, error) {
    db := s.poolManager.ForWrite()  // Always primary
    // ... existing mutation logic ...
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L8 (Store) | **HIGH** | Cache interface, Redis adapter, pool manager, invalidator |
| L5 (Component) | **MEDIUM** | New `leader/` component for election |
| L6 (Runner) | **MEDIUM** | Leader wrapper, conditional execution |
| L10 (Infra) | **LOW** | Redis connection config, PG NOTIFY triggers |
| L2 (API GW) | **LOW** | Health endpoint reports leader/cache status |
| L4 (Service) | **NONE** | Store interface unchanged — transparent |
| L1 (Frontend) | **NONE** | No UI changes needed |

---

## 4. Prometheus Metrics

**File**: `backend/store/cache_metrics.go` (new)

```go
var (
    cacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_cache_hits_total",
        Help: "Total cache hits",
    }, []string{"cache_name"})

    cacheMissesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_cache_misses_total",
        Help: "Total cache misses",
    }, []string{"cache_name"})

    cacheSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_cache_entries",
        Help: "Current number of entries in cache",
    }, []string{"cache_name"})

    leaderStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_leader_status",
        Help: "1 if this replica is leader for the runner, 0 otherwise",
    }, []string{"runner_name"})

    poolOpenConns = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_db_pool_open_connections",
        Help: "Open connections per pool",
    }, []string{"pool_name"})  // "primary", "replica"

    replicaLagSeconds = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_db_replica_lag_seconds",
        Help: "Read replica lag in seconds",
    })
)
```

---

## 5. Migration Safety Plan

### 5.1 Rollout Steps

```
Phase A (Sprint 1-2):
  1. Merge Cache interface + LRU adapter (no behavior change)
  2. Refactor Store to use Cache[K,V] interface
  3. Add Redis adapter + factory
  4. Add PG NOTIFY triggers
  5. Add CacheInvalidator runner
  6. Test: single-node unchanged, HA+Redis cache hits ≥ 85%

Phase B (Sprint 3):
  7. Add LeaderElector component
  8. Add LeaderRunner wrapper
  9. Wire into server.go for HA mode
  10. Test: 3-replica cluster, verify only 1 runs each runner

Phase C (Sprint 4):
  11. Add PoolManager with replica support
  12. Route read queries to replica
  13. Test: read replica lag monitoring, fallback behavior
```

### 5.2 Rollback Plan

```
Phase A rollback: Set REDIS_URL="" → falls back to no-cache HA (existing behavior)
Phase B rollback: Set HA=false → runners start without leader election
Phase C rollback: Set PG_READ_REPLICA_URL="" → all queries hit primary
```

All phases are **independently reversible** via environment variables.

---

## 6. Performance Validation

### 6.1 Expected Improvement

| Metric                      | Current (HA, no cache) | Phase A (Redis) | Phase B (leaders) | Phase C (replicas) |
|-----------------------------|------------------------|-----------------|--------------------|--------------------|
| P50 query latency           | ~15ms                  | ~3ms            | ~3ms               | ~3ms               |
| P99 query latency           | ~80ms                  | ~12ms           | ~12ms              | ~8ms               |
| Max concurrent users        | ~50-100                | ~300-500        | ~300-500           | ~500+              |
| DB primary load             | 100%                   | ~40%            | ~35%               | ~20%               |
| Runner duplicate work       | N × replicas           | N × replicas    | 1×                 | 1×                 |
| Cache hit ratio             | 0% (disabled)          | ≥ 85%           | ≥ 85%              | ≥ 85%              |

### 6.2 Load Test Plan

```bash
# Tool: k6 or vegeta
# Scenario: 500 concurrent users, mixed read/write (80/20)
# Duration: 30 minutes sustained load
# Targets: P99 < 20ms, error rate < 0.1%, cache hit > 85%

# Phase A validation:
k6 run --vus=500 --duration=30m test_cache_performance.js

# Phase B validation:
# Run 3 replicas, verify via Prometheus:
# bytebase_leader_status{runner_name="TaskScheduler"} == 1 on exactly 1 replica
```

---

## 7. Configuration Reference

| Variable               | Default        | Phase | Description                          |
|------------------------|----------------|-------|--------------------------------------|
| `REDIS_URL`            | _(empty)_      | A     | Redis URL for distributed cache      |
| `REDIS_PASSWORD`       | _(empty)_      | A     | Redis auth password                  |
| `REDIS_TLS`            | `false`        | A     | Enable TLS for Redis                 |
| `CACHE_DEFAULT_TTL`    | `600s`         | A     | Default cache entry TTL              |
| `LEADER_RENEW_INTERVAL`| `10s`         | B     | Advisory lock renewal interval       |
| `PG_READ_REPLICA_URL`  | _(empty)_      | C     | Read replica connection URL          |
| `REPLICA_LAG_THRESHOLD`| `5s`           | C     | Max acceptable replica lag           |
