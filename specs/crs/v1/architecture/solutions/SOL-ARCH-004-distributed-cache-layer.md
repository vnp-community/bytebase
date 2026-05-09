# Solution: Distributed Cache Layer — CR-ARCH-004

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-004                                             |
| **CR Reference**   | CR-ARCH-004                                              |
| **Title**          | Cache Abstraction + Redis/Valkey Adapter for HA Mode     |
| **Affected Layers**| L8 (Store)                                               |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §8 (L8 — Store Layer):
- 13 LRU caches: userEmailCache, instanceCache, databaseCache, projectCache, policyCache, settingCache, rolesCache, groupCache, groupMembersCache, memberGroupsCache, dbSchemaCache, iamPolicyCache, sheetFullCache

Per [TDD.md](../../TDD.md) §4.2:
> "HA mode: Cache disabled — mỗi request đọc trực tiếp từ DB"

---

## 2. Current Implementation Analysis

### 2.1 Cache Initialization (store.go:43-106)

```go
func New(ctx context.Context, pgURL string, enableCache bool) (*Store, error) {
    userEmailCache, _ := lru.New[string, *UserMessage](32768)
    // ...13 caches initialized unconditionally...
    s := &Store{
        enableCache: enableCache,   // ← !profile.HA
        userEmailCache: userEmailCache,
        // ...
    }
}
```

### 2.2 Cache Usage Pattern (example from user.go)

```go
func (s *Store) GetUser(ctx context.Context, find *FindUserMessage) (*UserMessage, error) {
    if s.enableCache {
        if v, ok := s.userEmailCache.Get(cacheKey); ok {
            return v, nil   // ← Cache hit (0.01ms)
        }
    }
    // ... DB query (5ms) ...
    if s.enableCache {
        s.userEmailCache.Add(cacheKey, result)
    }
    return result, nil
}
```

**Problem**: `enableCache = !profile.HA` → HA mode skips ALL cache reads/writes.

---

## 3. Solution Design

### 3.1 Phase 1 — Cache Abstraction Interface

**New file**: `backend/store/cache/cache.go`

```go
package cache

import (
    "context"
    "time"
)

// Cache is the generic cache interface used by Store.
// Implementations: LRU (in-process), Redis (distributed), Noop (disabled).
type Cache[K comparable, V any] interface {
    // Get retrieves a value by key. Returns (value, true, nil) on hit.
    Get(ctx context.Context, key K) (V, bool, error)
    // Set stores a value with optional TTL. Zero TTL = no expiry.
    Set(ctx context.Context, key K, value V, ttl time.Duration) error
    // Delete removes a key from cache.
    Delete(ctx context.Context, key K) error
    // Purge removes all entries from cache.
    Purge(ctx context.Context) error
}

// Backend identifies the cache implementation type.
type Backend string

const (
    BackendLRU   Backend = "lru"
    BackendRedis Backend = "redis"
    BackendNoop  Backend = "noop"
)
```

### 3.2 Phase 1b — LRU Adapter (Wrap Existing)

**New file**: `backend/store/cache/lru.go`

```go
package cache

import (
    "context"
    "time"

    lru "github.com/hashicorp/golang-lru/v2"
    "github.com/hashicorp/golang-lru/v2/expirable"
)

// LRUCache wraps hashicorp/golang-lru to satisfy Cache interface.
// This is the EXISTING behavior, just behind an interface.
type LRUCache[K comparable, V any] struct {
    inner *lru.Cache[K, V]
}

func NewLRU[K comparable, V any](capacity int) (*LRUCache[K, V], error) {
    inner, err := lru.New[K, V](capacity)
    if err != nil {
        return nil, err
    }
    return &LRUCache[K, V]{inner: inner}, nil
}

func (c *LRUCache[K, V]) Get(_ context.Context, key K) (V, bool, error) {
    v, ok := c.inner.Get(key)
    return v, ok, nil
}

func (c *LRUCache[K, V]) Set(_ context.Context, key K, value V, _ time.Duration) error {
    c.inner.Add(key, value)
    return nil
}

func (c *LRUCache[K, V]) Delete(_ context.Context, key K) error {
    c.inner.Remove(key)
    return nil
}

func (c *LRUCache[K, V]) Purge(_ context.Context) error {
    c.inner.Purge()
    return nil
}

// ExpirableLRUCache wraps expirable.LRU to satisfy Cache interface.
type ExpirableLRUCache[K comparable, V any] struct {
    inner *expirable.LRU[K, V]
}

func NewExpirableLRU[K comparable, V any](capacity int, ttl time.Duration) *ExpirableLRUCache[K, V] {
    return &ExpirableLRUCache[K, V]{
        inner: expirable.NewLRU[K, V](capacity, nil, ttl),
    }
}

func (c *ExpirableLRUCache[K, V]) Get(_ context.Context, key K) (V, bool, error) {
    v, ok := c.inner.Get(key)
    return v, ok, nil
}

func (c *ExpirableLRUCache[K, V]) Set(_ context.Context, key K, value V, _ time.Duration) error {
    c.inner.Add(key, value)
    return nil
}

func (c *ExpirableLRUCache[K, V]) Delete(_ context.Context, key K) error {
    c.inner.Remove(key)
    return nil
}

func (c *ExpirableLRUCache[K, V]) Purge(_ context.Context) error {
    c.inner.Purge()
    return nil
}
```

### 3.3 Phase 2 — Redis Adapter

**New file**: `backend/store/cache/redis.go`

```go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// RedisCache implements Cache using Redis/Valkey for distributed cache sharing.
type RedisCache[K comparable, V any] struct {
    client *redis.Client
    prefix string
    codec  Codec[V]
}

// Codec handles serialization for cache values.
type Codec[V any] struct {
    Marshal   func(V) ([]byte, error)
    Unmarshal func([]byte) (V, error)
}

// JSONCodec returns a codec using JSON serialization.
func JSONCodec[V any]() Codec[V] {
    return Codec[V]{
        Marshal: func(v V) ([]byte, error) { return json.Marshal(v) },
        Unmarshal: func(b []byte) (V, error) {
            var v V
            err := json.Unmarshal(b, &v)
            return v, err
        },
    }
}

func NewRedis[K comparable, V any](client *redis.Client, prefix string, codec Codec[V]) *RedisCache[K, V] {
    return &RedisCache[K, V]{
        client: client,
        prefix: prefix,
        codec:  codec,
    }
}

func (c *RedisCache[K, V]) key(k K) string {
    return fmt.Sprintf("%s:%v", c.prefix, k)
}

func (c *RedisCache[K, V]) Get(ctx context.Context, k K) (V, bool, error) {
    var zero V
    data, err := c.client.Get(ctx, c.key(k)).Bytes()
    if err == redis.Nil {
        return zero, false, nil  // cache miss
    }
    if err != nil {
        // Redis error → treat as miss (graceful degradation)
        return zero, false, nil
    }
    v, err := c.codec.Unmarshal(data)
    if err != nil {
        return zero, false, nil
    }
    return v, true, nil
}

func (c *RedisCache[K, V]) Set(ctx context.Context, k K, v V, ttl time.Duration) error {
    data, err := c.codec.Marshal(v)
    if err != nil {
        return err
    }
    return c.client.Set(ctx, c.key(k), data, ttl).Err()
}

func (c *RedisCache[K, V]) Delete(ctx context.Context, k K) error {
    return c.client.Del(ctx, c.key(k)).Err()
}

func (c *RedisCache[K, V]) Purge(ctx context.Context) error {
    // Use SCAN to find all keys with prefix, then DEL
    iter := c.client.Scan(ctx, 0, c.prefix+":*", 100).Iterator()
    var keys []string
    for iter.Next(ctx) {
        keys = append(keys, iter.Val())
    }
    if len(keys) > 0 {
        return c.client.Del(ctx, keys...).Err()
    }
    return nil
}
```

### 3.4 Phase 2b — Noop Cache (Testing/Disabled)

**New file**: `backend/store/cache/noop.go`

```go
package cache

import (
    "context"
    "time"
)

// NoopCache always misses — used for testing or when cache is disabled.
type NoopCache[K comparable, V any] struct{}

func NewNoop[K comparable, V any]() *NoopCache[K, V] {
    return &NoopCache[K, V]{}
}

func (c *NoopCache[K, V]) Get(_ context.Context, _ K) (V, bool, error) {
    var zero V
    return zero, false, nil
}

func (c *NoopCache[K, V]) Set(_ context.Context, _ K, _ V, _ time.Duration) error { return nil }
func (c *NoopCache[K, V]) Delete(_ context.Context, _ K) error                    { return nil }
func (c *NoopCache[K, V]) Purge(_ context.Context) error                          { return nil }
```

### 3.5 Phase 3 — Store Integration

**Modified file**: `backend/store/store.go`

```go
type Store struct {
    dbConnManager *DBConnectionManager

    // Cache — uses Cache[K,V] interface instead of concrete LRU
    userEmailCache cache.Cache[string, *UserMessage]
    instanceCache  cache.Cache[string, *InstanceMessage]
    databaseCache  cache.Cache[string, *DatabaseMessage]
    projectCache   cache.Cache[string, *ProjectMessage]
    policyCache    cache.Cache[string, *PolicyMessage]
    settingCache   cache.Cache[string, *SettingMessage]
    rolesCache     cache.Cache[string, *RoleMessage]
    // ... remaining caches
}

func New(ctx context.Context, pgURL string, cacheBackend cache.Backend, redisURL string) (*Store, error) {
    switch cacheBackend {
    case cache.BackendLRU:
        // Existing behavior — in-process LRU
        userEmailCache, _ := cache.NewLRU[string, *UserMessage](32768)
        // ...
    case cache.BackendRedis:
        // HA mode — distributed cache
        redisClient := redis.NewClient(&redis.Options{Addr: redisURL})
        userEmailCache := cache.NewRedis[string, *UserMessage](
            redisClient, "bb:user", cache.JSONCodec[*UserMessage](),
        )
        // ...
    case cache.BackendNoop:
        // Disabled
        userEmailCache := cache.NewNoop[string, *UserMessage]()
        // ...
    }
}
```

**Modified usage** (example user.go):

```go
func (s *Store) GetUser(ctx context.Context, find *FindUserMessage) (*UserMessage, error) {
    // Cache check — works for LRU, Redis, or Noop transparently
    if v, ok, _ := s.userEmailCache.Get(ctx, cacheKey); ok {
        return v, nil
    }
    // ... DB query ...
    s.userEmailCache.Set(ctx, cacheKey, result, 2*time.Minute)
    return result, nil
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/store/cache/cache.go` | L8 | **NEW** | Cache interface |
| `backend/store/cache/lru.go` | L8 | **NEW** | LRU adapter |
| `backend/store/cache/redis.go` | L8 | **NEW** | Redis adapter |
| `backend/store/cache/noop.go` | L8 | **NEW** | Noop adapter |
| `backend/store/store.go` | L8 | **MODIFY** | Use Cache interface |
| `backend/store/user.go` | L8 | **MODIFY** | Cache.Get/Set signature |
| `backend/component/config/profile.go` | L5 | **MODIFY** | Cache backend config |
| `go.mod` | — | **MODIFY** | Add `github.com/redis/go-redis/v9` |

---

## 5. Dependency Direction Validation

```
L8 (store.go) → cache.Cache (L8 interface — same layer)
L8 (cache/redis.go) → github.com/redis/go-redis/v9 (external)
L8 (cache/lru.go) → hashicorp/golang-lru (existing dep)
```

**New dependency**: `go-redis/v9` — only imported when `BackendRedis` is used.

---

## 6. Migration Strategy

### Config-Driven Cache Selection

```env
# Single-node (default): in-process LRU
CACHE_BACKEND=lru

# HA mode: distributed Redis
CACHE_BACKEND=redis
CACHE_REDIS_URL=redis://redis:6379

# Testing/disabled
CACHE_BACKEND=noop
```

### Server Integration (server.go:140)

```go
// BEFORE:
stores, err := store.New(ctx, pgURL, !profile.HA)

// AFTER:
cacheBackend := cache.BackendLRU
if profile.HA {
    if profile.CacheRedisURL != "" {
        cacheBackend = cache.BackendRedis
    } else {
        cacheBackend = cache.BackendNoop  // HA without Redis = no cache
    }
}
stores, err := store.New(ctx, pgURL, cacheBackend, profile.CacheRedisURL)
```

---

## 7. Rollback Plan

1. Set `CACHE_BACKEND=lru` → original single-node behavior
2. Set `CACHE_BACKEND=noop` → HA mode without Redis (current behavior)
3. Redis adapter errors → graceful degradation (returns cache miss)
