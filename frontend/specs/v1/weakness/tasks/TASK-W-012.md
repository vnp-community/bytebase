# TASK-W-012: Cache Eviction Engine

> **Source**: SOL-WEAK-002 §3.3 | **Priority**: P2 | **Effort**: 2.5h  
> **Status**: DONE | **Deps**: W-011

## Scope
- **NEW** `src/store/cache-eviction.ts`

## What
Implement `CacheEvictionEngine` class: periodic TTL sweep (60s via `requestIdleCallback`), LRU size enforcement per namespace, configurable per-namespace settings.

## Implementation — see SOL-WEAK-002 §3.1, §3.3
- `CacheConfig` interface: `{ maxSize, ttlMs, trace }`
- `DEFAULT_CONFIG`: maxSize=500, ttlMs=5min, trace=false
- `NAMESPACE_CONFIGS`: per-namespace overrides (database=1000/10min, auth=10/never, etc.)
- `sweepAll()`: iterate ENTITY_CACHE, delete entries where `now - createdAt > ttlMs`
- `enforceSizeLimit(ns, map)`: sort by `lastAccessedAt`, delete oldest
- `start()` / `stop()`: setInterval + requestIdleCallback

## AC
- [ ] File created at `src/store/cache-eviction.ts`
- [ ] TTL sweep runs every 60 seconds
- [ ] LRU eviction triggered after `setEntity`
- [ ] Per-namespace config (auth never expires, database 10min)
- [ ] Engine exportable as singleton
