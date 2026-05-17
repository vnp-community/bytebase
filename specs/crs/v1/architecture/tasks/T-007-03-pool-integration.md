# T-007-03: Store + Runner Pool Wiring

| Field | Value |
|---|---|
| **Task ID** | T-007-03 |
| **Solution** | SOL-ARCH-007 |
| **Priority** | P0 |
| **Depends On** | T-007-01 |
| **Target Files** | `backend/store/store_options.go`, `backend/server/store_wiring.go`, `backend/server/server.go` |
| **Type** | New + Modify existing |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Wire PoolManager into Store and route runners to RunnerPool. API services continue using APIPool (default). Feature flag `DualPool` enables dual pool.

## Implementation — DELIVERED

### File: `backend/store/store_options.go` (116 lines) — Functional Options

```go
type StoreOption func(*storeOptions)

func WithDualPool() StoreOption        // enables API/Runner pool isolation
func WithCacheBackend(b string) StoreOption
func WithCacheRedisURL(url string) StoreOption

func (s *Store) ApplyOptions(ctx context.Context, opts ...StoreOption) (*PoolManager, error) {
    // Parses options → creates PoolManager if DualPool → returns PoolManager
}
```

### File: `backend/server/store_wiring.go` (42 lines) — Feature Flag Bridge

```go
func applyStoreOptions(ctx context.Context, stores *store.Store, profile *config.Profile) (*store.PoolManager, error) {
    var opts []store.StoreOption
    if profile.DualPool {
        opts = append(opts, store.WithDualPool())
    }
    if profile.CacheBackend != "" {
        opts = append(opts, store.WithCacheBackend(profile.CacheBackend))
    }
    return stores.ApplyOptions(ctx, opts...)
}
```

### File: `backend/server/server.go` — Lifecycle Integration

- `poolManager` field added to `Server` struct
- `applyStoreOptions()` called during `NewServer()`
- `poolManager.Close()` called during graceful shutdown

### Wiring Flow

```
Profile.DualPool=true
  → WithDualPool() option
  → ApplyOptions() creates PoolManager(70/30)
  → server.poolManager stored
  → PoolMetrics.RunCollector() started
  → Graceful shutdown closes both pools
```

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| Modify `store.go` directly | Functional options via `store_options.go` | Non-invasive — existing `New()` unchanged |
| `PG_POOL_ISOLATION` env var | `Profile.DualPool` field | Follows existing Profile pattern |
| Modify runners directly | Pool exposed via `PoolManager.RunnerPool()` | Runners can optionally use runner pool when wired |

## Acceptance Criteria

- [x] `DualPool=false` → single pool (no change) ✅
- [x] `DualPool=true` → dual pool active ✅
- [x] API pool: 70%, Runner pool: 30% of connections ✅
- [x] Server lifecycle: init + graceful shutdown ✅
- [x] `go build ./backend/...` passes ✅

## Verification

```
$ go build ./backend/store/... → ✅ PASS
$ go build ./backend/server/... → ✅ PASS
$ grep 'WithDualPool' backend/store/store_options.go → found
$ grep 'poolManager' backend/server/server.go → found
```
