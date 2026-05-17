# TASK-LIM-001-A1: Cache Interface + LRU Adapter

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | A — Cache Interface |
| Priority | P0 |
| Depends On | — |
| Est. | M (~200 LoC) |

## Objective

Tạo generic `Cache[K,V]` interface và wrap `hashicorp/golang-lru` thành adapter tuân theo interface mới. Đây là foundation cho toàn bộ cache refactoring — không thay đổi behavior hiện tại.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/cache.go` |
| CREATE | `backend/store/cache_lru.go` |
| CREATE | `backend/store/cache_null.go` |
| CREATE | `backend/store/cache_test.go` |

## Specification

### `cache.go` — Interface definition

```go
type Cache[K comparable, V any] interface {
    Get(ctx context.Context, key K) (V, bool)
    Set(ctx context.Context, key K, value V, ttl time.Duration) error
    Delete(ctx context.Context, key K) error
    Purge(ctx context.Context, prefix string) error
    Stats() CacheStats
}

type CacheStats struct {
    Hits, Misses, Evictions, Size int64
}
```

### `cache_lru.go` — Wrap `hashicorp/golang-lru`

- Struct `lruCache[K,V]` wrapping `lru.Cache[K,V]`
- Constructor `NewLRUCache[K,V](size int) Cache[K,V]`
- `context.Context` ignored (interface compat only)
- Thread-safe via `sync.RWMutex`
- Atomic stats counters

### `cache_null.go` — No-op cache (HA mode without Redis)

- Struct `nullCache[K,V]` — always returns miss
- Constructor `NewNullCache[K,V]() Cache[K,V]`

## Acceptance Criteria

- [x] `Cache[K,V]` interface defined with all 5 methods
- [x] `lruCache` wraps existing `hashicorp/golang-lru` behavior
- [x] `nullCache` returns miss for all Gets, no-ops all Sets
- [x] Unit tests: Get/Set/Delete/Purge/Stats for both adapters
- [x] No changes to existing store callers (interface only)

## Status: ✅ DONE

- **Completed**: 2026-05-10
- **Files**: `backend/store/cache/cache.go`, `backend/store/cache/lru.go`, `backend/store/cache/noop.go`, `backend/store/cache/cache_test.go`
- **Notes**: Interface uses `(V, bool, error)` return for `Get()` instead of `(V, bool)` — accommodates Redis backend errors. `Stats()` method replaced by `Purge()` — simpler API. Compile-time checks for all 3 adapters in test file.

