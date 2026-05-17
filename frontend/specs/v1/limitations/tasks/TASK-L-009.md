# TASK-L-009: Sync Store Reset on Auth Routes

> **Source**: SOL-LIM-006 §2.3 | **Priority**: P1 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/router/index.ts` (auth route handler)

## What
Await async store reset (AI conversation) trước khi navigate, tránh race condition khi data cũ còn trong store.

## Implementation

```diff
 if (isAuthRelatedRoute(to.name as string)) {
   useDatabaseV1Store().reset();
   useProjectV1Store().reset();
   useInstanceV1Store().reset();
+  // Async reset — await BEFORE navigation
+  try {
+    const { useConversationStore } = await import("@/plugins/ai/store");
+    useConversationStore().reset();
+  } catch {
+    // AI plugin may not be loaded — safe to ignore
+  }
   next();
   return;
 }
```

## AC
- [ ] AI conversation store reset is awaited before `next()`
- [ ] Missing AI plugin doesn't block navigation (catch protects)
- [ ] Synchronous store resets (database, project, instance) unchanged
