# TASK-W-008: Refactor ReactPageMount.vue

> **Source**: SOL-WEAK-001 §3.2 | **Priority**: P1 | **Effort**: 3h  
> **Status**: DONE | **Deps**: W-007

## Scope
- **EDIT** `src/react/ReactPageMount.vue`

## What
Replace complex async render queue with delegation to `bridgeManager`. Remove `renderQueue` + error-swallowing `.catch(() => undefined)`.

## Implementation — see SOL-WEAK-001 §3.2
```diff
-let root: Root | null = null;
-let renderQueue = Promise.resolve();
-function render() { ... renderQueue = next.catch(() => undefined); ... }
+import { bridgeManager } from "./BridgeLifecycleManager";
+const abortController = new AbortController();
+
+watch(() => [props.page, pageProps.value], async () => {
+  try {
+    await bridgeManager.mount(container.value, { pageName: props.page, props: pageProps.value }, abortController.signal);
+  } catch (error) {
+    useNotificationStore().pushNotification({ module: "bytebase", style: "CRITICAL", title: `Failed to load page: ${props.page}`, description: String(error) });
+  }
+}, { immediate: true });
+
+onUnmounted(() => { abortController.abort(); bridgeManager.unmountCurrent(); });
```

## AC
- [ ] No more `renderQueue` variable
- [ ] No more `.catch(() => undefined)` error swallowing
- [ ] Mount errors show notification to user
- [ ] `onUnmounted` aborts pending mount and unmounts root
- [ ] All existing React pages still mount correctly
