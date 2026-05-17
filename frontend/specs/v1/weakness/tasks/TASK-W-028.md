# TASK-W-028: Optimize Route Watcher

> **Source**: SOL-WEAK-006 §2.4 | **Priority**: P3 | **Effort**: 1h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/App.vue` (~L163-181)

## What
Replace `cloneDeep` + `isEqual` (lodash) with shallow field check for query parameter preservation.

## Implementation — see SOL-WEAK-006 §2.4 diff
```diff
-import { cloneDeep, isEqual } from "lodash-es";
+const PRESERVED_FIELDS = ["mode", "customTheme", "lang", "project", "filter"] as const;

 watch(route, (current, prev) => {
-  const preservedQuery = cloneDeep(current.query);
-  // ... lodash deep comparison
+  let needsUpdate = false;
+  const updates: Record<string, string> = {};
+  for (const field of PRESERVED_FIELDS) {
+    if (!(field in current.query) && prev.query[field]) {
+      updates[field] = prev.query[field] as string;
+      needsUpdate = true;
+    }
+  }
+  if (!needsUpdate) return;
+  router.replace({ ...current, query: { ...current.query, ...updates } });
 });
```

## AC
- [x] No `cloneDeep` or `isEqual` import for this watcher
- [x] Query params still preserved across navigations
- [x] Reduced CPU per navigation event
