# TASK-L-016: Route-Scoped Query Preservation

> **Source**: SOL-LIM-006 §2.1 | **Priority**: P2 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/App.vue` (query preservation watcher)
- **EDIT** `src/router/dashboard/workspace.ts` (add route meta)

## What
Thay thế global query preservation (5 hardcoded params) bằng route-scoped whitelist via `meta.preserveQuery`. Dùng `flush: "post"` + `nextTick` chống infinite loop.

## Implementation

### Change 1: Add RouteMeta type augmentation
```typescript
// src/router/index.ts hoặc shims.d.ts
declare module "vue-router" {
  interface RouteMeta {
    preserveQuery?: string[];
  }
}
```

### Change 2: Update App.vue watcher
```diff
-const PRESERVED_FIELDS = ["project", "filter", "sort", "page", "view"];
-watch(route, (current, prev) => {
-  const query = { ...current.query };
-  let changed = false;
-  for (const field of PRESERVED_FIELDS) {
-    if (query[field] === undefined && prev.query[field] !== undefined) {
-      query[field] = prev.query[field];
-      changed = true;
-    }
-  }
-  if (changed) router.replace({ ...current, query });
-});

+watch(route, (current, prev) => {
+  const preservable = current.meta.preserveQuery;
+  if (!preservable || preservable.length === 0) return;
+  const preservedQuery = { ...current.query };
+  let changed = false;
+  for (const key of preservable) {
+    if (preservedQuery[key] === undefined && prev.query[key] !== undefined) {
+      preservedQuery[key] = prev.query[key];
+      changed = true;
+    }
+  }
+  if (!changed) return;
+  nextTick(() => {
+    router.replace({ ...current, query: preservedQuery });
+  });
+}, { flush: "post" });
```

### Change 3: Add preserveQuery to relevant routes
```typescript
// router/dashboard/workspace.ts
{
  name: "workspace.databases",
  meta: { preserveQuery: ["project", "filter"] },
}
```

## AC
- [ ] Query params only preserved when declared in route meta
- [ ] Undeclared query params do NOT carry over between routes
- [ ] No infinite loop (flush: post + nextTick)
- [ ] Existing filter persistence behavior maintained for declared routes
