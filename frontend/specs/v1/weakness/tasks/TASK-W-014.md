# TASK-W-014: Consolidate Database Triple Cache

> **Source**: SOL-WEAK-002 §3.5 | **Priority**: P2 | **Effort**: 2.5h  
> **Status**: DONE | **Deps**: W-013

## Scope
- **EDIT** `src/store/modules/v1/database.ts`

## What
Remove standalone `databaseRequestCache` Map (~L157). Replace all usages with unified `useCache` API.

## Implementation — see SOL-WEAK-002 §3.5 diff
```diff
-const databaseRequestCache = new Map<string, Promise<Database>>();

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

## AC
- [ ] `databaseRequestCache` variable removed
- [ ] All database fetches go through `useCache` API
- [ ] Failed fetch requests can be retried (no cached rejected promise)
- [ ] Database CRUD still works correctly
