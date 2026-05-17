# TASK-L-014: Namespace Tier Map + useCache Update

> **Source**: SOL-LIM-002 §2.1+2.3 | **Priority**: P2 | **Effort**: 2h  
> **Status**: DONE | **Deps**: L-013

## Scope
- **EDIT** `src/store/cache.ts` (NAMESPACE_TIER_MAP + useCache function)

## What
Thêm tier configuration map và update `useCache()` để sử dụng `LRUEntityCache` thay vì `Map`. API giữ nguyên backward compatible.

## Implementation

```typescript
// Thêm tier defaults và namespace mapping
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
```

```diff
 // Update useCache to use LRUEntityCache
 export const useCache = <K extends KeyType[], T>(namespace: string) => {
   const tier = NAMESPACE_TIER_MAP[namespace] || "standard";
   const config = TIER_DEFAULTS[tier];
-  const entityCacheMap = getEntityCacheMap<K, T>(namespace);
+  const entityCache = getOrCreateLRUCache<T>(namespace, config);

-  const getEntity = (keys: K) => entityCacheMap.get(getKey(keys));
-  const setEntity = (keys: K, entity: T) => entityCacheMap.set(getKey(keys), entity);
+  const getEntity = (keys: K) => entityCache.get(getKey(keys));
+  const setEntity = (keys: K, entity: T) => entityCache.set(getKey(keys), entity);

   // ... rest of API unchanged
 };
```

## AC
- [ ] All 33 domain stores work without code changes (backward compatible)
- [ ] `dbSchema` namespace uses heavy tier (max 30 entries, 5min TTL)
- [ ] `database` namespace uses standard tier (max 200 entries, 10min TTL)
- [ ] Unknown namespaces default to "standard" tier
