# SOL-WEAK-002: Cache Eviction Engine — Bounded Growth & Stale Data Prevention

> **Source**: [BUG-WEAK-002](../bugs/BUG-WEAK-002-cache-unbounded-growth.md)  
> **Severity**: MEDIUM-HIGH → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Thiết kế lại cache với: TTL-based eviction, LRU size cap, consolidated cache API (loại bỏ `databaseRequestCache`).

---

## 2. Thay đổi Kiến trúc

### Architecture Doc Section 7.2
- **Trước**: Dual-layer cache (no TTL, no eviction)
- **Sau**: Managed cache: Request + Entity + TTL + LRU + Health Monitor

### TDD Section 3.3.2
- Thêm `CacheConfig`, eviction policies, health API

---

## 3. Thiết kế Chi tiết

### 3.1 Cache Configuration

```typescript
interface CacheConfig {
  maxSize: number;    // LRU eviction threshold
  ttlMs: number;      // 0 = no expiry
  trace: boolean;     // dev-only debug
}

const DEFAULT_CONFIG: CacheConfig = {
  maxSize: 500,
  ttlMs: 5 * 60 * 1000,
  trace: false,  // ← Fix BUG 2.4: no more production debug spam
};

const NAMESPACE_CONFIGS: Record<string, Partial<CacheConfig>> = {
  database:     { maxSize: 1000, ttlMs: 10 * 60_000 },
  project:      { maxSize: 200,  ttlMs: 10 * 60_000 },
  instance:     { maxSize: 100,  ttlMs: 15 * 60_000 },
  auth:         { maxSize: 10,   ttlMs: 0 },   // never expire
  subscription: { maxSize: 5,    ttlMs: 0 },
};
```

### 3.2 Enhanced Entity Cache Entry

```typescript
interface EntityCacheEntry<T> {
  value: T;
  createdAt: number;
  lastAccessedAt: number;  // for LRU
}
```

### 3.3 Eviction Engine

```typescript
class CacheEvictionEngine {
  private intervalId: ReturnType<typeof setInterval> | null = null;

  start(): void {
    this.intervalId = setInterval(() => {
      requestIdleCallback?.(() => this.sweepAll(), { timeout: 5000 })
        ?? this.sweepAll();
    }, 60_000);
  }

  sweepAll(): void {
    const now = Date.now();
    for (const [ns, map] of ENTITY_CACHE) {
      const ttl = getConfig(ns).ttlMs;
      if (ttl === 0) continue;
      for (const [key, entry] of map) {
        if (now - entry.createdAt > ttl) map.delete(key);
      }
    }
  }

  enforceSizeLimit(ns: string, map: Map<string, EntityCacheEntry<unknown>>): void {
    const max = getConfig(ns).maxSize;
    if (map.size <= max) return;
    const sorted = [...map.entries()].sort((a, b) => a[1].lastAccessedAt - b[1].lastAccessedAt);
    for (let i = 0; i < map.size - max; i++) map.delete(sorted[i][0]);
  }

  stop(): void { clearInterval(this.intervalId!); this.intervalId = null; }
}

export const evictionEngine = new CacheEvictionEngine();
```

### 3.4 Enhanced `useCache`

Key changes:
- `getEntity()`: TTL check on read, update `lastAccessedAt`
- `setEntity()`: call `evictionEngine.enforceSizeLimit()` after insert
- `setRequest()`: auto-cleanup on **reject** (fix BUG 2.2 — rejected promises cached forever)
- `trace()`: noop function in production (fix BUG 2.4)

```typescript
const setRequest = <T>(keys: KeyType, promise: Promise<T>) => {
  const ac = new AbortController();
  getRequestMap(ns).set(getKey(keys), { promise, abortController: ac });
  promise
    .then(() => invalidateRequest(keys))
    .catch(() => invalidateRequest(keys));  // ← NEW: cleanup on reject
  return promise;
};
```

### 3.5 Consolidate Triple Cache (Fix BUG 2.1, 2.3)

```diff
// src/store/modules/v1/database.ts
-const databaseRequestCache = new Map<string, Promise<Database>>();
 const { getRequest, setRequest, getEntity, setEntity } = useCache<[string], Database>("database");

 const getOrFetchDatabaseByName = async (name: string) => {
   const cached = getEntity([name]);
   if (cached) return cached;
-  const hit = databaseRequestCache.get(name);
+  const hit = getRequest([name]);
   if (hit) return hit;
   const req = fetchDatabaseByName(name);
-  databaseRequestCache.set(name, req);
+  setRequest([name], req);
   return req;
 };
```

### 3.6 Dev Health Monitor

```typescript
// window.__BB_CACHE_STATS__() in DevTools console
if (isDev()) {
  (window as any).__BB_CACHE_STATS__ = () => {
    const stats = {};
    for (const [ns, map] of ENTITY_CACHE) {
      stats[ns] = { entities: map.size, requests: REQUEST_CACHE.get(ns)?.size ?? 0 };
    }
    console.table(stats);
  };
}
```

---

## 4. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | Add timestamps to `EntityCacheEntry` | LOW | 2h |
| 2 | Implement `CacheEvictionEngine` | LOW | 3h |
| 3 | Enhance `useCache` (TTL, auto-cleanup) | MEDIUM | 4h |
| 4 | Remove `databaseRequestCache` | MEDIUM | 3h |
| 5 | Per-namespace config + health monitor | LOW | 2h |

**Total**: ~14h (2 days)

---

## 5. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Max cache entries (1h session) | Unbounded | ≤500/namespace |
| Stale data after deletion | Persists until reload | Auto-evicted after TTL |
| Failed request retry | Blocked (cached reject) | Cleaned, allows retry |
| Console.debug in production | Active | Disabled (noop) |
| Cache layers (database) | 3 (triple) | 2 (unified) |
