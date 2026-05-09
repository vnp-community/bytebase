# T-004-01: Cache Interface Definition

| Field | Value |
|---|---|
| **Task ID** | T-004-01 |
| **Solution** | SOL-ARCH-004 |
| **Priority** | P1 |
| **Depends On** | None |
| **Target File** | `backend/store/cache/cache.go` |
| **Type** | New file |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Define generic `Cache[K, V]` interface for Store layer. Supports multiple backends (LRU, Redis, Noop).

## Implementation — DELIVERED

### File: `backend/store/cache/cache.go` (45 lines)

### Core Interface

```go
type Cache[K comparable, V any] interface {
    Get(ctx context.Context, key K) (V, bool, error)
    Set(ctx context.Context, key K, value V, ttl time.Duration) error
    Delete(ctx context.Context, key K) error
    Purge(ctx context.Context) error
}
```

### Supporting Types

| Type | Definition | Purpose |
|------|-----------|---------|
| `Backend` | `type Backend string` | Identifies cache implementation |
| `BackendLRU` | `"lru"` | In-process LRU cache (default) |
| `BackendRedis` | `"redis"` | Distributed Redis/Valkey cache |
| `BackendNoop` | `"noop"` | Disabled caching (testing/debug) |
| `Codec[V]` | `interface { Marshal(V) ([]byte, error); Unmarshal([]byte) (V, error) }` | Serialization for Redis adapter |

## Acceptance Criteria

- [x] `Cache[K,V]` interface with Get/Set/Delete/Purge ✅
- [x] `Backend` type with 3 variants (lru, redis, noop) ✅
- [x] `Codec[V]` interface for serialization ✅
- [x] `go build ./backend/store/cache/...` passes ✅

## Verification

```
$ go build ./backend/store/cache/... → ✅ PASS
$ wc -l backend/store/cache/cache.go → 45
```
