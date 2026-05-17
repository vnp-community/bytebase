# TASK-W-013: Enhanced useCache Hook

> **Source**: SOL-WEAK-002 §3.4 | **Priority**: P2 | **Effort**: 3h  
> **Status**: DONE | **Deps**: W-011, W-012

## Scope
- **EDIT** `src/store/cache.ts`

## What
1. `getEntity()`: add TTL check on read (return undefined if expired)
2. `setEntity()`: call `evictionEngine.enforceSizeLimit()` after insert
3. `setRequest()`: auto-cleanup on **reject** (not just resolve)
4. Add `getStats()` method for dev health monitor

## Implementation — key change for rejected promises:
```diff
 const setRequest = <T>(keys: KeyType, promise: Promise<T>) => {
   const ac = new AbortController();
   getRequestMap(ns).set(getKey(keys), { promise, abortController: ac });
   promise
     .then(() => invalidateRequest(keys))
-    // (no catch — rejected promises stay cached forever)
+    .catch(() => invalidateRequest(keys));
   return promise;
 };
```

## AC
- [ ] Expired entities return `undefined` from `getEntity`
- [ ] `setEntity` triggers LRU check
- [ ] Rejected request promises cleaned from cache
- [ ] `getStats()` returns `{ namespace, entityCount, requestCount, maxSize, ttlMs }`
