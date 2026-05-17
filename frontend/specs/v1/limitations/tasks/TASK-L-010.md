# TASK-L-010: Render Versioning in ReactPageMount

> **Source**: SOL-LIM-001 §2.1 | **Priority**: P2 | **Effort**: 3h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/react/ReactPageMount.vue` (render function)

## What
Thay thế serialization queue bằng generation counter. Capture props snapshot đồng bộ trước async. Validate DOM connectivity sau mỗi yield point.

## Implementation

```diff
-// Existing renderQueue approach
-const renderQueue = Promise.resolve();
-function render() { renderQueue = renderQueue.then(doRender); }

+let renderGeneration = 0;
+
+async function render() {
+  const gen = ++renderGeneration;
+  const snapshotPage = props.page;
+  const snapshotProps = pageProps.value ? { ...pageProps.value } : undefined;
+
+  const [{ mountReactPage, updateReactPage }, i18nModule] = await Promise.all([
+    import("./mount"),
+    import("./i18n"),
+  ]);
+
+  // Guard: stale render or unmounted
+  if (unmounted || gen !== renderGeneration || !container.value) return;
+  if (!container.value.isConnected) return;
+
+  // Sync locale
+  if (i18nModule.default.language !== locale.value) {
+    await i18nModule.default.changeLanguage(locale.value);
+    if (gen !== renderGeneration || unmounted || !container.value?.isConnected) return;
+  }
+
+  // Page changed → full remount
+  if (root && currentPage !== snapshotPage) {
+    root.unmount();
+    root = null;
+  }
+
+  if (!root) {
+    root = await mountReactPage(container.value, snapshotPage, snapshotProps);
+    if (gen !== renderGeneration || unmounted) {
+      root?.unmount();
+      root = null;
+      return;
+    }
+  } else {
+    await updateReactPage(root, snapshotPage, snapshotProps);
+  }
+  currentPage = snapshotPage;
+}
```

## AC
- [ ] Rapid page switching (A→B→C) only mounts C
- [ ] Props snapshot captured before first `await`
- [ ] `container.value.isConnected` checked after each yield
- [ ] `gen !== renderGeneration` aborts stale renders
