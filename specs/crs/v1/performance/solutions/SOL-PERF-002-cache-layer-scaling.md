# Solution: CR-PERF-002 — Cache Layer Scaling

| Field          | Value                                    |
|----------------|------------------------------------------|
| **CR Ref**     | CR-PERF-002                              |
| **Solution ID**| SOL-PERF-002                             |
| **Status**     | Proposed                                 |
| **Created**    | 2026-05-08                               |
| **Arch Refs**  | L8 (Data Access Layer)                   |
| **TDD Refs**   | §4.1 Store Architecture, §4.2 Cache Strategy |

---

## 1. Solution Overview

Redesign cache layer trong `backend/store/store.go` để hỗ trợ 200K+ databases. Giải pháp:
- **Adaptive LRU sizing** dựa trên actual entity count
- **Compressed schema cache** (L2 tier) cho dbSchemaCache
- **Tenant-aware eviction** qua weighted LRU
- **Background cache warming** on startup

Giữ nguyên `hashicorp/golang-lru` library (proven, in-use) — chỉ thay đổi configuration và thêm wrapper layer.

---

## 2. Detailed Technical Design

### 2.1 Adaptive Cache Sizing

**File**: `backend/store/store.go`

```go
// BEFORE (hardcoded):
databaseCache, err := lru.New[string, *DatabaseMessage](32768)
dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](128, nil, 5*time.Minute)

// AFTER (adaptive):
func New(ctx context.Context, pgURL string, enableCache bool) (*Store, error) {
    // ... connection setup ...

    // Query actual counts for adaptive sizing
    dbCount := getEntityCount(ctx, db, "db", "deleted = false")
    instanceCount := getEntityCount(ctx, db, "instance", "deleted = false")

    // Adaptive sizing: cache 50% of entities, min 32768, max 500000
    dbCacheSize := adaptiveCacheSize(dbCount, 32768, 500000, 50)
    schemaCacheSize := adaptiveCacheSize(dbCount, 512, 50000, 10)
    instanceCacheSize := adaptiveCacheSize(instanceCount, 1024, 32768, 80)

    databaseCache, err := lru.New[string, *DatabaseMessage](dbCacheSize)
    // ...
    dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](
        schemaCacheSize, nil, 10*time.Minute,  // increased TTL
    )
    // ...
}

func adaptiveCacheSize(entityCount, minSize, maxSize, coveragePct int) int {
    target := entityCount * coveragePct / 100
    if target < minSize { return minSize }
    if target > maxSize { return maxSize }
    return target
}

func getEntityCount(ctx context.Context, db *sql.DB, table, condition string) int {
    var count int
    query := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", table, condition)
    if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
        return 0 // fallback to min size
    }
    return count
}
```

### 2.2 Compressed Schema Cache (L2 Tier)

Hiện tại `dbSchemaCache` chỉ 128 entries vì `model.DatabaseMetadata` rất lớn (~50KB-500KB per entry). Giải pháp: thêm L2 cache lưu proto bytes đã compress.

**File**: `backend/store/cache_compressed.go` (new)

```go
package store

import (
    "bytes"
    "compress/gzip"
    "sync"

    "github.com/hashicorp/golang-lru/v2/expirable"
    "google.golang.org/protobuf/proto"

    storepb "github.com/bytebase/bytebase/backend/generated-go/store"
    "github.com/bytebase/bytebase/backend/store/model"
)

// CompressedSchemaCache provides a memory-efficient L2 cache for database schemas.
// L1 (expirable.LRU) stores deserialized objects (fast, limited capacity).
// L2 (this) stores gzip-compressed proto bytes (slow decode, 3-5x smaller).
type CompressedSchemaCache struct {
    mu    sync.RWMutex
    cache *expirable.LRU[string, []byte]  // key → compressed proto bytes
}

func NewCompressedSchemaCache(size int, ttl time.Duration) *CompressedSchemaCache {
    return &CompressedSchemaCache{
        cache: expirable.NewLRU[string, []byte](size, nil, ttl),
    }
}

func (c *CompressedSchemaCache) Get(key string) (*model.DatabaseMetadata, bool) {
    c.mu.RLock()
    compressed, ok := c.cache.Get(key)
    c.mu.RUnlock()
    if !ok {
        return nil, false
    }

    // Decompress
    reader, err := gzip.NewReader(bytes.NewReader(compressed))
    if err != nil {
        return nil, false
    }
    defer reader.Close()

    var buf bytes.Buffer
    if _, err := buf.ReadFrom(reader); err != nil {
        return nil, false
    }

    // Unmarshal proto
    metadata := &storepb.DatabaseSchemaMetadata{}
    if err := proto.Unmarshal(buf.Bytes(), metadata); err != nil {
        return nil, false
    }

    return model.NewDatabaseMetadataFromProto(metadata), true
}

func (c *CompressedSchemaCache) Add(key string, metadata *model.DatabaseMetadata) {
    protoBytes, err := proto.Marshal(metadata.GetProto())
    if err != nil {
        return
    }

    // Compress
    var buf bytes.Buffer
    writer := gzip.NewWriter(&buf)
    writer.Write(protoBytes)
    writer.Close()

    c.mu.Lock()
    c.cache.Add(key, buf.Bytes())
    c.mu.Unlock()
}
```

### 2.3 Tiered Cache Lookup in `GetDBSchema`

**File**: `backend/store/db_schema.go`

```go
func (s *Store) GetDBSchema(ctx context.Context, find *FindDBSchemaMessage) (*model.DatabaseMetadata, error) {
    cacheKey := getDBSchemaCacheKey(find.InstanceID, find.DatabaseName)

    // L1: Fast in-memory (small, hot entries)
    if v, ok := s.dbSchemaCache.Get(cacheKey); ok {
        return v, nil
    }

    // L2: Compressed cache (larger capacity, slower decode)
    if s.dbSchemaL2Cache != nil {
        if v, ok := s.dbSchemaL2Cache.Get(cacheKey); ok {
            // Promote to L1
            s.dbSchemaCache.Add(cacheKey, v)
            return v, nil
        }
    }

    // L3: Database query (slowest)
    result, err := s.queryDBSchema(ctx, find)
    if err != nil {
        return nil, err
    }
    if result != nil {
        s.dbSchemaCache.Add(cacheKey, result)
        if s.dbSchemaL2Cache != nil {
            s.dbSchemaL2Cache.Add(cacheKey, result)
        }
    }
    return result, nil
}
```

### 2.4 Store Constructor Update

**File**: `backend/store/store.go`

```go
type Store struct {
    // ... existing fields ...

    // L2 compressed schema cache (new)
    dbSchemaL2Cache *CompressedSchemaCache

    // Cache metrics
    cacheMetrics *CacheMetrics
}

func New(ctx context.Context, pgURL string, enableCache bool) (*Store, error) {
    // ... existing code ...

    // Adaptive sizing
    dbCount := getEntityCount(ctx, db, "db", "deleted = false")

    dbCacheSize := adaptiveCacheSize(dbCount, 32768, 500000, 50)
    schemaL1Size := adaptiveCacheSize(dbCount, 512, 5000, 2)
    schemaL2Size := adaptiveCacheSize(dbCount, 5000, 100000, 25)

    databaseCache, _ := lru.New[string, *DatabaseMessage](dbCacheSize)
    dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](
        schemaL1Size, nil, 10*time.Minute,
    )
    dbSchemaL2Cache := NewCompressedSchemaCache(schemaL2Size, 30*time.Minute)

    s := &Store{
        // ... existing fields ...
        databaseCache:   databaseCache,
        dbSchemaCache:   dbSchemaCache,
        dbSchemaL2Cache: dbSchemaL2Cache,
        cacheMetrics:    NewCacheMetrics(),
    }
    return s, nil
}
```

### 2.5 Cache Metrics (Prometheus)

**File**: `backend/store/cache_metrics.go` (new)

```go
package store

import "github.com/prometheus/client_golang/prometheus"

type CacheMetrics struct {
    hits   *prometheus.CounterVec
    misses *prometheus.CounterVec
    size   *prometheus.GaugeVec
}

func NewCacheMetrics() *CacheMetrics {
    m := &CacheMetrics{
        hits: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_cache_hits_total",
            Help: "Total cache hits by cache name and tier",
        }, []string{"cache", "tier"}),
        misses: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_cache_misses_total",
            Help: "Total cache misses by cache name",
        }, []string{"cache"}),
        size: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_cache_entries",
            Help: "Current number of entries in cache",
        }, []string{"cache", "tier"}),
    }
    prometheus.MustRegister(m.hits, m.misses, m.size)
    return m
}
```

### 2.6 Background Cache Warming

**File**: `backend/store/cache_warmer.go` (new)

```go
package store

import (
    "context"
    "log/slog"
    "time"
)

// WarmDatabaseCache pre-loads recently accessed databases into cache on startup.
func (s *Store) WarmDatabaseCache(ctx context.Context) {
    if !s.enableCache {
        return
    }

    start := time.Now()
    limit := 10000

    // Load most recently synced databases (likely most accessed)
    databases, err := s.ListDatabases(ctx, &FindDatabaseMessage{
        Limit: &limit,
        OrderByKeys: []*OrderByKey{
            {Key: "db.metadata->>'lastSyncTime'", SortOrder: DESC},
        },
    })
    if err != nil {
        slog.Warn("Cache warming failed", "error", err)
        return
    }

    slog.Info("Cache warming completed",
        "databases", len(databases),
        "duration", time.Since(start),
    )
}
```

---

## 3. Impact on Architecture Layers

| Layer | Impact | Details |
|-------|--------|---------|
| L8 (Store) | **HIGH** | New cache types, adaptive sizing, metrics |
| L10 (Infra) | **LOW** | Prometheus metrics registration |
| L6 (Runner) | **LOW** | Benefits from warmer cache during sync |
| Others | **NONE** | Cache is internal to Store |

---

## 4. Memory Estimation

| Cache | Entries | Per-Entry Size | Total Memory |
|-------|---------|---------------|--------------|
| databaseCache (200K×50%) | 100K | ~2KB | ~200MB |
| dbSchemaCache L1 | 2K | ~100KB | ~200MB |
| dbSchemaCache L2 (compressed) | 25K | ~30KB | ~750MB |
| instanceCache | 2K | ~5KB | ~10MB |
| projectCache | 1K | ~3KB | ~3MB |
| **Total** | — | — | **~1.2GB** |

Memory footprint stays under 2GB target with generous margins.

---

## 5. Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `CACHE_AUTO_SCALE` | `true` | Enable adaptive cache sizing |
| `CACHE_DB_MAX_SIZE` | `500000` | Max database cache entries |
| `CACHE_SCHEMA_L2_ENABLED` | `true` | Enable compressed L2 schema cache |
| `CACHE_WARM_ON_STARTUP` | `true` | Pre-warm cache on startup |
| `CACHE_WARM_COUNT` | `10000` | Databases to pre-warm |
