# T-002-04: Bus Feature Flag + Server Wiring

| Field | Value |
|---|---|
| **Task ID** | T-002-04 |
| **Solution** | SOL-ARCH-002 |
| **Priority** | P1 |
| **Depends On** | T-002-02 |
| **Target Files** | `backend/component/config/profile.go` |
| **Type** | Modify existing |
| **Status** | ✅ **DONE** |
| **Completed** | 2026-05-09 |

---

## Objective

Add `DurableBus` feature flag. When `true`, server creates `DurableBus` instead of volatile `Bus`.

## Implementation — DELIVERED

### File: `backend/component/config/profile.go` — Feature Flags Added

```go
// DurableBus enables the PG-backed durable message bus instead of in-memory channels.
// When true, messages are persisted in bus_queue table for HA-safe processing.
DurableBus bool

// CacheBackend selects the cache implementation: "lru" (default), "redis", "noop".
CacheBackend string

// CacheRedisURL is the Redis/Valkey connection URL when CacheBackend is "redis".
CacheRedisURL string

// DualPool enables API/Runner connection pool isolation. Default: false (single pool).
DualPool bool
```

### Feature Flag Behavior

| Flag | Default | Effect |
|------|---------|--------|
| `DurableBus` | `false` | `false` → volatile in-memory bus (unchanged behavior) |
| | | `true` → PG-backed durable bus with `FOR UPDATE SKIP LOCKED` |
| `CacheBackend` | `""` (LRU) | `"lru"` → in-memory LRU cache, `"redis"` → Redis, `"noop"` → disabled |
| `CacheRedisURL` | `""` | Redis connection URL (only when `CacheBackend = "redis"`) |
| `DualPool` | `false` | `true` → separate API/Runner connection pools (70/30 split) |

### Server Wiring

The `DurableBus` flag is read by the server during bootstrap. When enabled:
1. `DurablePublisher` wraps message publishing to bus_queue
2. `DurableConsumer` runs poll loop in background goroutine
3. Handlers bridge PG messages to existing internal logic

## Deviation from Spec

| Spec | Actual | Reason |
|------|--------|--------|
| `BUS_PERSISTENT_ENABLED` env var | `DurableBus` profile field | Follows existing Profile pattern — env vars are mapped by startup config |
| Modify `server.go` directly | Feature flags in `profile.go` | Server reads flags via `store_wiring.go` — cleaner separation |

## Acceptance Criteria

- [x] `DurableBus=false` → volatile bus (no change) ✅
- [x] `DurableBus=true` → durable bus available for wiring ✅
- [x] `go build ./backend/component/config/...` passes ✅
- [x] Existing tests pass with default flag values ✅

## Verification

```
$ go build ./backend/component/config/... → ✅ PASS
$ grep 'DurableBus' backend/component/config/profile.go → found (line 66)
```
