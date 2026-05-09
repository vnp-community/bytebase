# TASK-LIM-001-A2: Redis Cache Adapter

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | A — Redis Adapter |
| Priority | P0 |
| Depends On | TASK-LIM-001-A1 |
| Est. | M (~180 LoC) |

## Objective

Implement `Cache[K,V]` interface using `github.com/redis/go-redis/v9`. Graceful degradation on Redis errors (treat as cache miss).

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/cache_redis.go` |
| CREATE | `backend/store/cache_redis_test.go` |
| MODIFY | `go.mod` — add `github.com/redis/go-redis/v9` |

## Specification

### `cache_redis.go`

```go
type redisCache[K comparable, V any] struct {
    client     redis.UniversalClient
    prefix     string              // namespace: "bb:user:", "bb:inst:"
    defaultTTL time.Duration
    marshal    func(V) ([]byte, error)
    unmarshal  func([]byte) (V, error)
}
```

Key behaviors:
- `Get`: `client.Get()` → unmarshal; `redis.Nil` → miss; other error → miss + slog.Warn
- `Set`: marshal → `client.Set()` with TTL
- `Delete`: `client.Del()`
- `Purge`: `client.Scan()` + `client.Del()` (no blocking KEYS)
- Key format: `{prefix}{key}` e.g. `bb:user:alice@example.com`
- Serialization: `protojson.Marshal/Unmarshal` for proto types, `encoding/json` for others

Constructor:
```go
func NewRedisCache[K comparable, V any](
    client redis.UniversalClient,
    prefix string,
    defaultTTL time.Duration,
    marshal func(V) ([]byte, error),
    unmarshal func([]byte) (V, error),
) Cache[K, V]
```

## Acceptance Criteria

- [ ] Implements all 5 `Cache[K,V]` methods
- [ ] Graceful degradation: Redis errors → cache miss (not application error)
- [ ] SCAN-based Purge (no KEYS blocking)
- [ ] Unit tests with redis mock or testcontainers
- [ ] `go.mod` updated with go-redis dependency
