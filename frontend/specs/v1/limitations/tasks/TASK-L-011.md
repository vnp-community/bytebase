# TASK-L-011: Cleanup Lifecycle Hardening

> **Source**: SOL-LIM-001 §2.2 | **Priority**: P2 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: L-010

## Scope
- **EDIT** `src/react/ReactPageMount.vue` (onUnmounted hook)

## What
Trong onUnmounted: increment `renderGeneration` để cancel in-flight renders, set `unmounted = true`, unmount React root.

## Implementation

```diff
 onUnmounted(() => {
   unmounted = true;
+  renderGeneration++; // Cancel mọi render đang in-flight
   root?.unmount();
   root = null;
 });
```

## AC
- [ ] Unmount during async load → no React root created
- [ ] `renderGeneration++` aborts any pending `doRefreshWithRetry` calls
- [ ] No console errors on rapid mount/unmount
