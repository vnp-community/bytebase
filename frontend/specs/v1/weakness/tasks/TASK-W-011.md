# TASK-W-011: Cache Entry Timestamps

> **Source**: SOL-WEAK-002 §3.1-3.2 | **Priority**: P2 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/store/cache.ts`

## What
Add `createdAt` and `lastAccessedAt` timestamps to `EntityCacheEntry`. Update `getEntity` to record access time, `setEntity` to record creation time.

## Implementation
```diff
-// Current EntityCacheEntry (implicit — just stores value)
+interface EntityCacheEntry<T> {
+  value: T;
+  createdAt: number;
+  lastAccessedAt: number;
+}

 // In setEntity:
-map.set(key, value);
+map.set(key, { value, createdAt: Date.now(), lastAccessedAt: Date.now() });

 // In getEntity:
-return map.get(key);
+const entry = map.get(key);
+if (entry) { entry.lastAccessedAt = Date.now(); return entry.value; }
+return undefined;
```

Also: wrap `console.debug` calls behind `isDev()` check or make trace a configurable noop:
```diff
-const trace = (title, keys, ...args) => {
-  console.debug("cache", namespace, title, JSON.stringify(keys), ...args);
-};
+const trace = isDev()
+  ? (title: string, keys: KeyType[], ...args: unknown[]) =>
+      console.debug("cache", namespace, title, JSON.stringify(keys), ...args)
+  : () => {};
```

## AC
- [ ] `EntityCacheEntry` has `createdAt` and `lastAccessedAt`
- [ ] `getEntity` updates `lastAccessedAt`
- [ ] `setEntity` sets both timestamps
- [ ] `console.debug` is noop in production
