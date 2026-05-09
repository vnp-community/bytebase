# T-004-02: LRU + Noop Cache Adapters

| Field | Value |
|---|---|
| **Task ID** | T-004-02 |
| **Solution** | SOL-ARCH-004 |
| **Priority** | P1 |
| **Depends On** | T-004-01 |
| **Target Files** | `backend/store/cache/lru.go`, `backend/store/cache/noop.go` |
| **Type** | New files |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Wrap existing `hashicorp/golang-lru` into Cache interface. Create Noop adapter for testing/disabled mode.

## Implementation — DELIVERED

### File: `backend/store/cache/lru.go` (46 lines)

```go
type LRUCache[K comparable, V any] struct {
    inner *lru.Cache[K, V]
}

func NewLRU[K comparable, V any](capacity int) (*LRUCache[K, V], error)
func (c *LRUCache[K, V]) Get(_ context.Context, key K) (V, bool, error)
func (c *LRUCache[K, V]) Set(_ context.Context, key K, value V, _ time.Duration) error
func (c *LRUCache[K, V]) Delete(_ context.Context, key K) error
func (c *LRUCache[K, V]) Purge(_ context.Context) error
```

- Wraps `hashicorp/golang-lru/v2` (already in go.mod — no new deps)
- `context.Context` accepted but unused (in-process, no I/O)
- TTL parameter ignored (LRU evicts by capacity, not time)

### File: `backend/store/cache/noop.go` (36 lines)

```go
type NoopCache[K comparable, V any] struct{}

func NewNoop[K comparable, V any]() *NoopCache[K, V]
func (c *NoopCache[K, V]) Get(context.Context, K) (V, bool, error) // always miss
func (c *NoopCache[K, V]) Set(context.Context, K, V, time.Duration) error // no-op
func (c *NoopCache[K, V]) Delete(context.Context, K) error // no-op
func (c *NoopCache[K, V]) Purge(context.Context) error // no-op
```

### Unit Tests: `cache_test.go` (4 tests)

| Test | Adapter | Validates |
|------|---------|-----------|
| `TestLRU_SetGet` | LRU | Store + retrieve |
| `TestLRU_Purge` | LRU | Purge clears all entries |
| `TestNoop_AlwaysMiss` | Noop | Get always returns false |
| `TestNoop_SetNoop` | Noop | Set doesn't error, Get still misses |

## Acceptance Criteria

- [x] `LRUCache` wraps existing lru, satisfies `Cache[K,V]` ✅
- [x] `NoopCache` always returns miss ✅
- [x] No new dependencies (uses existing `hashicorp/golang-lru`) ✅
- [x] Unit tests for both adapters (4 tests, all pass) ✅

## Verification

```
$ go test ./backend/store/cache/... → ok (0.524s) ✅
$ wc -l backend/store/cache/lru.go → 46
$ wc -l backend/store/cache/noop.go → 36
```
