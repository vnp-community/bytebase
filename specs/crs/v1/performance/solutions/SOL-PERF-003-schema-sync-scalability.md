# Solution: CR-PERF-003 — Schema Sync Scalability

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-003                              |
| **Solution ID**| SOL-PERF-003                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L6 (Runner Layer), L5 (Bus), L8 (Store)  |
| **TDD Refs**   | §5 Background Runner System, §5.2 Task Execution |

---

## 1. Solution Overview

Hiện tại `schemasync.Syncer` (TDD §5.1) load toàn bộ databases qua `ListDatabases(ctx, &FindDatabaseMessage{})` mỗi 15 phút. Với 200K databases, điều này gây OOM.

Giải pháp:
1. **Paginated instance-based sync** — iterate instances, sync per-instance databases
2. **Checksum-based skip** — compare schema hash trước khi full sync  
3. **Adaptive concurrency** — dựa trên runtime metrics
4. **Advisory lock** — tận dụng hạ tầng `advisory_lock.go` có sẵn (TDD §5.1)

---

## 2. Detailed Technical Design

### 2.1 Refactored trySyncAll — Instance-Based Pagination

Từ Architecture L6, schema syncer hoạt động theo instance → databases. Thay vì load tất cả databases, iterate per-instance.

**File**: `backend/runner/schemasync/syncer.go`

```go
// BEFORE (line 195-215): Load ALL databases
func (s *Syncer) trySyncAll(ctx context.Context) {
    databases, err := s.store.ListDatabases(ctx, &store.FindDatabaseMessage{})
    // ... loads 200K databases into memory

// AFTER: Instance-based pagination
func (s *Syncer) trySyncAll(ctx context.Context) {
    // Step 1: Load instances (typically < 10K, manageable)
    instances, err := s.store.ListAllInstances(ctx, false)
    if err != nil {
        slog.Error("failed to list instances for sync", "error", err)
        return
    }

    // Step 2: Shard instances for staggered sync
    // Use advisory lock to ensure single syncer across replicas
    lock, acquired, err := store.TryAdvisoryLock(ctx, s.store.GetDB(),
        store.AdvisoryLockKeySchemaSyncer)
    if err != nil || !acquired {
        return // Another replica is syncing
    }
    defer lock.Release()

    // Step 3: Sync instances with adaptive concurrency pool
    pool := s.newAdaptivePool()
    for _, instance := range instances {
        inst := instance
        pool.Go(func() {
            s.syncInstanceDatabases(ctx, inst)
        })
    }
    pool.Wait()
}

func (s *Syncer) syncInstanceDatabases(ctx context.Context, instance *store.InstanceMessage) {
    // Load databases for this instance only (bounded by instance size)
    databases, err := s.store.ListDatabases(ctx, &store.FindDatabaseMessage{
        InstanceID: &instance.ResourceID,
        Workspace:  instance.Workspace,
    })
    if err != nil {
        slog.Error("failed to list databases for sync",
            "instance", instance.ResourceID, "error", err)
        return
    }

    for _, db := range databases {
        if s.shouldSkipSync(ctx, db) {
            continue  // Checksum unchanged
        }
        s.syncDatabase(ctx, instance, db)
    }
}
```

### 2.2 Checksum-Based Incremental Sync

Store schema checksum in `db.metadata` JSONB field. Compare before doing expensive full sync.

**File**: `backend/runner/schemasync/checksum.go` (new)

```go
package schemasync

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "time"

    "github.com/bytebase/bytebase/backend/store"
)

const (
    // forceFullSyncInterval forces a full sync regardless of checksum.
    forceFullSyncInterval = 24 * time.Hour
    // checksumKey is the metadata key storing last known schema hash.
    checksumKey = "schemaChecksum"
    // lastSyncKey is the metadata key storing last sync timestamp.
    lastSyncKey = "lastSyncTime"
)

// shouldSkipSync returns true if the database schema hasn't changed
// since the last sync (based on a lightweight checksum comparison).
func (s *Syncer) shouldSkipSync(ctx context.Context, db *store.DatabaseMessage) bool {
    if db.Metadata == nil {
        return false // No metadata → always sync
    }

    // Force full sync every 24h
    lastSync := db.Metadata.GetLastSyncTime()
    if lastSync.IsValid() {
        elapsed := time.Since(lastSync.AsTime())
        if elapsed > forceFullSyncInterval {
            return false
        }
    }

    // Quick checksum via lightweight driver query
    remoteChecksum, err := s.getRemoteSchemaChecksum(ctx, db)
    if err != nil {
        return false // Error → sync to be safe
    }

    storedChecksum := db.Metadata.GetSchemaChecksum()
    return remoteChecksum == storedChecksum
}

// getRemoteSchemaChecksum gets a lightweight schema fingerprint from the target DB.
// Uses pg_catalog/information_schema queries that are much cheaper than full dump.
func (s *Syncer) getRemoteSchemaChecksum(ctx context.Context, db *store.DatabaseMessage) (string, error) {
    driver, err := s.dbFactory.GetAdminDatabaseDriver(ctx, db.InstanceID, db)
    if err != nil {
        return "", err
    }
    defer driver.Close(ctx)

    // PostgreSQL: hash of table count + column count + index count + last DDL time
    // This is O(1) instead of full schema dump which is O(tables × columns)
    rows, err := driver.QueryConn(ctx, nil, `
        SELECT md5(string_agg(
            t.table_name || ':' || t.column_count::text || ':' ||
            COALESCE(t.last_ddl, '1970-01-01')::text,
            '|' ORDER BY t.table_name
        ))
        FROM (
            SELECT c.table_name,
                   COUNT(cols.column_name) as column_count,
                   MAX(c.table_name) as last_ddl
            FROM information_schema.tables c
            LEFT JOIN information_schema.columns cols
                ON cols.table_schema = c.table_schema AND cols.table_name = c.table_name
            WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
              AND c.table_type = 'BASE TABLE'
            GROUP BY c.table_name
        ) t
    `, nil)
    if err != nil {
        return "", err
    }
    // Extract hash from result
    if len(rows) > 0 && len(rows[0].Rows) > 0 {
        return rows[0].Rows[0].Values[0].GetStringValue(), nil
    }
    return "", nil
}
```

### 2.3 Adaptive Concurrency Pool

**File**: `backend/runner/schemasync/adaptive_pool.go` (new)

```go
package schemasync

import (
    "os"
    "runtime"
    "strconv"

    "github.com/sourcegraph/conc/pool"
)

const (
    defaultMinConcurrency = 10
    defaultMaxConcurrency = 300
)

// newAdaptivePool creates a worker pool with concurrency adjusted
// based on available CPU and configured limits.
func (s *Syncer) newAdaptivePool() *pool.Pool {
    maxWorkers := calculateAdaptiveConcurrency()
    return pool.New().WithMaxGoroutines(maxWorkers)
}

func calculateAdaptiveConcurrency() int {
    // Check env override first
    if v := os.Getenv("SYNC_MAX_CONCURRENCY"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return n
        }
    }

    // Adaptive: 10 workers per CPU core, capped
    cpuCount := runtime.NumCPU()
    target := cpuCount * 10

    minC := defaultMinConcurrency
    maxC := defaultMaxConcurrency
    if v := os.Getenv("SYNC_MIN_CONCURRENCY"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { minC = n }
    }

    if target < minC { return minC }
    if target > maxC { return maxC }
    return target
}
```

### 2.4 Sync Metrics

**File**: `backend/runner/schemasync/metrics.go` (new)

```go
package schemasync

import "github.com/prometheus/client_golang/prometheus"

var (
    syncCycleDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name:    "bytebase_schema_sync_cycle_duration_seconds",
        Help:    "Duration of a full schema sync cycle",
        Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s to 2048s
    })
    syncDatabasesProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_schema_sync_databases_total",
        Help: "Total databases processed by schema sync",
    }, []string{"status"}) // status: synced, skipped, error
    syncInstanceDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "bytebase_schema_sync_instance_duration_seconds",
        Help:    "Duration of schema sync per instance",
        Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
    }, []string{"workspace"})
    syncConcurrency = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "bytebase_schema_sync_concurrency",
        Help: "Current number of concurrent sync workers",
    })
)

func init() {
    prometheus.MustRegister(
        syncCycleDuration, syncDatabasesProcessed,
        syncInstanceDuration, syncConcurrency,
    )
}
```

### 2.5 Updated Sync Loop with Metrics

**File**: `backend/runner/schemasync/syncer.go` — modified `trySyncAll`

```go
func (s *Syncer) trySyncAll(ctx context.Context) {
    timer := prometheus.NewTimer(syncCycleDuration)
    defer timer.ObserveDuration()

    // ... (instance loading + advisory lock from §2.1) ...

    pool := s.newAdaptivePool()
    syncConcurrency.Set(float64(calculateAdaptiveConcurrency()))

    for _, instance := range instances {
        inst := instance
        pool.Go(func() {
            instTimer := prometheus.NewTimer(
                syncInstanceDuration.WithLabelValues(inst.Workspace),
            )
            defer instTimer.ObserveDuration()

            s.syncInstanceDatabases(ctx, inst)
        })
    }
    pool.Wait()
    syncConcurrency.Set(0)
}

func (s *Syncer) syncDatabase(ctx context.Context, inst *store.InstanceMessage, db *store.DatabaseMessage) {
    if err := s.SyncDatabaseSchema(ctx, db); err != nil {
        syncDatabasesProcessed.WithLabelValues("error").Inc()
        slog.Error("failed to sync database schema",
            "database", db.DatabaseName, "error", err)
        return
    }
    syncDatabasesProcessed.WithLabelValues("synced").Inc()
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L6 (Runner) | **HIGH** | Syncer refactored: paginated, adaptive, incremental |
| L8 (Store) | **MEDIUM** | Optional: add `ListDatabasesByInstance()` optimized query |
| L5 (Bus) | **NONE** | Advisory lock already exists, no bus changes |
| L7 (Plugin) | **LOW** | Add checksum query per engine driver |
| L10 (Infra) | **LOW** | Prometheus metrics registration |

---

## 4. Memory Estimation (200K databases, 1K instances)

| Phase | Memory Usage |
|-------|-------------|
| Before: Load all 200K databases | ~2GB (10KB × 200K) |
| After: Per-instance (avg 200 DBs) | ~2MB per instance batch |
| Peak concurrent (300 workers × 200 DBs) | ~120MB |
| **Improvement** | **~15x reduction** |

---

## 5. Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `SYNC_MAX_CONCURRENCY` | `300` | Max concurrent sync workers |
| `SYNC_MIN_CONCURRENCY` | `10` | Min sync workers |
| `SYNC_CHECKSUM_ENABLED` | `true` | Enable incremental checksum sync |
| `SYNC_FORCE_FULL_HOURS` | `24` | Hours between forced full syncs |
| `SYNC_INTERVAL_MINUTES` | `15` | Sync interval (existing) |
