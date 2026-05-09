# TASK-LIM-001-A3: Store Cache Factory + Wiring

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-001 |
| Phase | A — Store Integration |
| Priority | P0 |
| Depends On | TASK-LIM-001-A1, TASK-LIM-001-A2 |
| Est. | M (~250 LoC) |

## Objective

Refactor `store.New()` to use `Cache[K,V]` interface. Add cache factory that selects LRU/Redis/Null based on config. Modify `server.go` to pass `REDIS_URL`.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/store/store.go` — `New()` signature + cache init |
| MODIFY | `backend/server/server.go` — pass redisURL to store |

## Specification

### `store.go` changes

Current: `New(ctx, pgURL, enableCache bool)` 
After: `New(ctx, pgURL string, enableCache bool, redisURL string)`

Add factory methods:
```go
func (s *Store) initLRUCaches() {
    s.userEmailCache = NewLRUCache[string, *UserMessage](1024)
    s.instanceCache = NewLRUCache[string, *InstanceMessage](256)
    // ... same sizes as current hardcoded values
}

func (s *Store) initRedisCaches(client redis.UniversalClient) {
    s.userEmailCache = NewRedisCache[string, *UserMessage](client, "bb:user:", 10*time.Minute, ...)
    s.instanceCache = NewRedisCache[string, *InstanceMessage](client, "bb:inst:", 10*time.Minute, ...)
}

func (s *Store) initNullCaches() {
    s.userEmailCache = NewNullCache[string, *UserMessage]()
    // ...
}
```

### `server.go` changes

```go
enableCache := true
redisURL := os.Getenv("REDIS_URL")
if profile.HA && redisURL == "" {
    enableCache = false  // backward compat
}
storeInstance, err := store.New(ctx, pgURL, enableCache, redisURL)
```

## Acceptance Criteria

- [ ] `store.New()` accepts `redisURL` parameter
- [ ] Single-node (no `REDIS_URL`): uses LRU cache — behavior identical to current
- [ ] HA + `REDIS_URL` set: uses Redis cache
- [ ] HA + no `REDIS_URL`: uses null cache (backward compat)
- [ ] Redis connection failure → fallback to null cache + warning log
- [ ] All existing store tests pass unchanged
