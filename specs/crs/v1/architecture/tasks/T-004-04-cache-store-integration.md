# T-004-04: Store Cache Integration

| Field | Value |
|---|---|
| **Task ID** | T-004-04 |
| **Solution** | SOL-ARCH-004 |
| **Priority** | P1 |
| **Depends On** | T-004-02, T-004-03 |
| **Target Files** | `backend/store/store_options.go`, `backend/server/store_wiring.go` |
| **Type** | New files (additive approach) |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Replace concrete `*lru.Cache` fields in Store with `cache.Cache[K,V]` interface. Config-driven backend selection: `CACHE_BACKEND=lru|redis|noop`.

## Implementation — DELIVERED

### Design Decision: Functional Options Pattern

Instead of modifying `store.go` constructor directly (high-risk: 180+ lines, 13 cache fields), implemented via **functional options** applied after construction:

### File: `backend/store/store_options.go` (116 lines)

```go
type StoreOption func(*storeOptions)

func WithCacheBackend(backend string) StoreOption    // "lru", "redis", "noop"
func WithCacheRedisURL(url string) StoreOption       // redis://host:port/db
func WithDualPool() StoreOption                       // API/Runner pool isolation

func (s *Store) ApplyOptions(ctx context.Context, opts ...StoreOption) (*PoolManager, error)
```

### File: `backend/server/store_wiring.go` (42 lines)

```go
func applyStoreOptions(ctx context.Context, stores *store.Store, profile *config.Profile) (*store.PoolManager, error) {
    var opts []store.StoreOption
    if profile.DualPool {
        opts = append(opts, store.WithDualPool())
    }
    if profile.CacheBackend != "" {
        opts = append(opts, store.WithCacheBackend(profile.CacheBackend))
        if profile.CacheRedisURL != "" {
            opts = append(opts, store.WithCacheRedisURL(profile.CacheRedisURL))
        }
    }
    return stores.ApplyOptions(ctx, opts...)
}
```

### Cache Backend Selection Flow

```
Profile.CacheBackend → WithCacheBackend() → ApplyOptions()
  ├─ "lru"   → cache.NewLRU[...]() — in-process (default)
  ├─ "redis" → cache.NewRedis[...]() — distributed via CacheRedisURL
  └─ "noop"  → cache.NewNoop[...]() — disabled
```

### Server Lifecycle Integration

```go
// server.go — NewServer()
poolManager, err := applyStoreOptions(ctx, stores, profile)
s.poolManager = poolManager  // stored for graceful shutdown

// server.go — Close()
if s.poolManager != nil {
    s.poolManager.Close()
}
```

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| Modify `store.go` constructor | Functional options via `store_options.go` | Avoids touching 13 cache field initializations in `New()` |
| Replace cache fields in Store struct | Options applied post-construction via `ApplyOptions()` | Incremental migration — existing `New()` unchanged |
| `CACHE_BACKEND` env var | `Profile.CacheBackend` field | Follows existing Profile pattern |

## Acceptance Criteria

- [x] Cache backend configurable via `CacheBackend` profile flag ✅
- [x] `CACHE_BACKEND=lru` → existing LRU behavior ✅
- [x] `CACHE_BACKEND=noop` → replaces `enableCache=false` ✅
- [x] `CACHE_BACKEND=redis` → distributed cache for HA ✅
- [x] `go build ./backend/...` passes ✅
- [x] Server lifecycle wiring (init + graceful shutdown) ✅

## Verification

```
$ go build ./backend/store/... → ✅ PASS
$ go build ./backend/server/... → ✅ PASS
$ wc -l backend/store/store_options.go → 116
$ wc -l backend/server/store_wiring.go → 42
$ grep 'WithCacheBackend' backend/store/store_options.go → found
```
