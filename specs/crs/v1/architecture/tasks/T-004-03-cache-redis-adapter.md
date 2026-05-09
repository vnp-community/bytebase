# T-004-03: Redis Cache Adapter

| Field | Value |
|---|---|
| **Task ID** | T-004-03 |
| **Solution** | SOL-ARCH-004 |
| **Priority** | P1 |
| **Depends On** | T-004-01 |
| **Target File** | `backend/store/cache/redis.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Redis/Valkey adapter for distributed cache in HA mode. Graceful degradation: Redis errors → cache miss (not failure).

## Implementation — DELIVERED

### File: `backend/store/cache/redis.go` (94 lines)

```go
type RedisCache[K comparable, V any] struct {
    client    *redis.Client
    codec     Codec[V]
    keyPrefix string
}

func NewRedis[K comparable, V any](url string, codec Codec[V], keyPrefix string) (*RedisCache[K, V], error)
func (c *RedisCache[K, V]) Get(ctx context.Context, key K) (V, bool, error)
func (c *RedisCache[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error
func (c *RedisCache[K, V]) Delete(ctx context.Context, key K) error
func (c *RedisCache[K, V]) Purge(ctx context.Context) error  // SCAN + DEL by prefix
func (c *RedisCache[K, V]) Close() error
```

### Key Design Decisions

| Aspect | Implementation | Rationale |
|--------|---------------|-----------|
| **Graceful degradation** | `redis.Nil` → `(zero, false, nil)` (cache miss, not error) | Redis unavailability shouldn't break the app |
| **Key namespacing** | `keyPrefix + ":" + fmt.Sprint(key)` | Multi-tenant safe, prevents key collisions |
| **Purge strategy** | `SCAN` + `DEL` with cursor pagination (batch 100) | Safe for large keyspaces, no `KEYS *` |
| **Connection** | `redis.ParseURL(url)` → `redis.NewClient(opts)` | Standard URL format: `redis://host:port/db` |
| **Serialization** | `Codec[V]` interface (Marshal/Unmarshal) | Decoupled from JSON — supports protobuf etc. |

### Graceful Degradation Flow

```
Get(key) → redis.Get()
  ├─ redis.Nil    → (zero, false, nil)  ← cache miss
  ├─ err != nil   → (zero, false, nil)  ← graceful: log + return miss
  └─ success      → codec.Unmarshal()   ← cache hit
```

## Acceptance Criteria

- [x] `RedisCache` satisfies `Cache[K,V]` ✅
- [x] Redis errors → graceful degradation (return miss, not error) ✅
- [x] `go-redis/v9` used for client (`redis.ParseURL`, `redis.NewClient`) ✅
- [x] `go build ./backend/store/cache/...` passes ✅

## Verification

```
$ go build ./backend/store/cache/... → ✅ PASS
$ wc -l backend/store/cache/redis.go → 94
$ grep 'redis.Nil' backend/store/cache/redis.go → found (graceful miss)
```
