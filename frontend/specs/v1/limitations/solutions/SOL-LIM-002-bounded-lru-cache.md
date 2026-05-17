# SOL-LIM-002 — Bounded LRU Cache With TTL and Namespace Tiers

> **Resolves**: BUG-LIM-002 (Entity Cache Tăng Trưởng Không Giới Hạn)  
> **Type**: Architectural Change (State Layer)  
> **Priority**: High  
> **Effort**: Medium (~1 tuần)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Thay thế `Map`-based entity cache bằng **bounded LRU cache** có TTL, phân chia theo namespace tiers để tối ưu memory cho 33 domain stores.

---

## 2. Giải Pháp Kiến Trúc

### 2.1 Cache Tiers (NEW — phân loại theo entity size)

| Tier | Namespaces | Max Entries | TTL | Rationale |
|---|---|---|---|---|
| **Heavy** | `dbSchema`, `databaseCatalog` | 30 | 5 min | ~50-200KB/entity, nên evict nhanh |
| **Standard** | `database`, `instance`, `project`, `issue`, `plan` | 200 | 10 min | ~3-10KB/entity |
| **Light** | `environment`, `role`, `group`, `policy`, `setting` | 500 | 30 min | ~1-3KB/entity, ít thay đổi |
| **Session** | `auth`, `subscription`, `workspace` | No limit | Session | Cần suốt session |

### 2.2 LRU Cache Implementation

```typescript
// src/store/cache.ts — NEW LRU implementation

interface CacheConfig {
  maxEntries: number;
  ttlMs: number;
  tier: "heavy" | "standard" | "light" | "session";
}

const TIER_DEFAULTS: Record<string, CacheConfig> = {
  heavy:    { maxEntries: 30,  ttlMs: 5 * 60_000,  tier: "heavy" },
  standard: { maxEntries: 200, ttlMs: 10 * 60_000, tier: "standard" },
  light:    { maxEntries: 500, ttlMs: 30 * 60_000, tier: "light" },
  session:  { maxEntries: Infinity, ttlMs: Infinity, tier: "session" },
};

const NAMESPACE_TIER_MAP: Record<string, string> = {
  dbSchema: "heavy", databaseCatalog: "heavy",
  database: "standard", instance: "standard", project: "standard",
  issue: "standard", plan: "standard", worksheet: "standard",
  environment: "light", role: "light", group: "light",
  policy: "light", setting: "light",
  auth: "session", subscription: "session", workspace: "session",
};

type TimestampedEntity<T> = {
  entity: T;
  accessedAt: number;
  createdAt: number;
};

class LRUEntityCache<K extends KeyType[], T> {
  private map: Map<string, TimestampedEntity<T>>;
  private config: CacheConfig;

  constructor(config: CacheConfig) {
    this.config = config;
    this.map = shallowReactive(new Map());
  }

  get(key: string): T | undefined {
    const entry = this.map.get(key);
    if (!entry) return undefined;

    // TTL check
    if (Date.now() - entry.createdAt > this.config.ttlMs) {
      this.map.delete(key);
      return undefined;
    }

    // LRU: Update access time (move to "end" by re-inserting)
    entry.accessedAt = Date.now();
    this.map.delete(key);
    this.map.set(key, entry);
    return entry.entity;
  }

  set(key: string, entity: T): void {
    // Evict oldest if at capacity
    if (this.map.size >= this.config.maxEntries && !this.map.has(key)) {
      const oldest = this.map.keys().next().value;
      if (oldest !== undefined) this.map.delete(oldest);
    }
    this.map.set(key, {
      entity,
      accessedAt: Date.now(),
      createdAt: Date.now(),
    });
  }

  delete(key: string): void { this.map.delete(key); }
  clear(): void { this.map.clear(); }
  get size(): number { return this.map.size; }
}
```

### 2.3 Updated `useCache()` API (Backward Compatible)

```typescript
export const useCache = <K extends KeyType[], T>(namespace: string) => {
  const tier = NAMESPACE_TIER_MAP[namespace] || "standard";
  const config = TIER_DEFAULTS[tier];
  const entityCache = getOrCreateLRUCache<K, T>(namespace, config);
  const requestCacheMap = getRequestCacheMap<K, T>(namespace);

  // API giữ nguyên — drop-in replacement
  const getEntity = (keys: K) => entityCache.get(getKey(keys));
  const setEntity = (keys: K, entity: T) => entityCache.set(getKey(keys), entity);
  const invalidateEntity = (keys: K) => {
    invalidateRequest(keys);
    entityCache.delete(getKey(keys));
  };
  const clear = () => {
    // Abort all flying requests
    for (const request of requestCacheMap.values()) {
      if (!request.abortController.signal.aborted) request.abortController.abort();
    }
    requestCacheMap.clear();
    entityCache.clear();
  };

  // ... getRequest, setRequest unchanged
  return { getRequest, getEntity, setRequest, setEntity, invalidateRequest, invalidateEntity, clear };
};
```

### 2.4 Dev-Mode Cache Monitor

```typescript
// src/store/cache-monitor.ts — NEW (dev only)
export function startCacheMonitor() {
  if (!isDev()) return;

  setInterval(() => {
    const stats: Record<string, number> = {};
    for (const [ns, cache] of ENTITY_CACHE.entries()) {
      stats[ns] = cache.size;
    }
    const total = Object.values(stats).reduce((a, b) => a + b, 0);
    if (total > 500) {
      console.warn("[Cache Monitor] High entity count:", total, stats);
    }
  }, 30_000); // Check every 30s
}
```

---

## 3. Thay Đổi Architecture Document

### 3.1 Cập nhật `specs/architecture.md` — Section 7.2 Cache Strategy

**Thay thế** nội dung hiện tại bằng:

> ### 7.2 Cache Strategy
>
> `cache.ts` triển khai **bounded LRU cache with TTL**, phân loại theo namespace tiers:
>
> | Tier | Max Entries | TTL | Namespaces |
> |---|---|---|---|
> | **Heavy** | 30 | 5 min | `dbSchema`, `databaseCatalog` |
> | **Standard** | 200 | 10 min | `database`, `instance`, `project`, `issue` |
> | **Light** | 500 | 30 min | `environment`, `role`, `group`, `policy` |
> | **Session** | Unlimited | Session | `auth`, `subscription`, `workspace` |
>
> - **Request Cache**: Deduplicate in-flight requests (AbortController-based)
> - **Entity Cache**: LRU eviction khi vượt max entries, TTL-based expiry
> - **Monitor**: Dev-mode cache monitor cảnh báo khi total entities > 500

### 3.2 Cập nhật `specs/technical-design-document.md` — Section 3.3.2 Cache Design

Thêm **Memory Budgeting** section:

> **Memory Budget**: Cache system giới hạn tổng memory footprint ~10-15MB qua LRU eviction. Heavy tier (schemas) evict sớm nhất (5 phút) vì entity size lớn. Session tier entities tồn tại suốt session vì chúng nhỏ và critical cho auth/navigation.

---

## 4. Migration Path

1. **Phase 1**: Thay thế `Map` bằng `LRUEntityCache` trong `cache.ts` — API giữ nguyên, 33 stores không cần đổi.
2. **Phase 2**: Thêm tier configuration cho mỗi namespace.
3. **Phase 3**: Enable cache monitor trong dev mode.
4. **Phase 4**: Add CI metrics tracking heap size trước/sau navigation workflows.

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Entity cache growth | Unbounded | Bounded per tier |
| Memory after 1h browsing 200 DBs | ~15-20MB | ~5-8MB (LRU eviction) |
| Cache miss rate (re-fetch) | 0% (never evict) | <5% for active entities |
| API compatibility | N/A | 100% backward compatible |
| dbSchema cache size | Unbounded | Max 30 entries (~6MB) |
