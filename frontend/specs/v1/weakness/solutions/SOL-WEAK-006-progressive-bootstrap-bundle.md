# SOL-WEAK-006: Progressive Bootstrap & Bundle Optimization

> **Source**: [BUG-WEAK-006](../bugs/BUG-WEAK-006-performance-bundle.md)  
> **Severity**: MEDIUM → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Chuyển từ blocking bootstrap sang progressive rendering, merge i18n, strip debug logs, optimize route watcher, lazy-load Monaco đúng cách.

---

## 2. Thiết kế Chi tiết

### 2.1 Progressive Bootstrap (Fix BUG 2.1)

**Current (blocking):**
```
fetchCurrentUser() → await → fetchServerInfo() → await → mount("#app")
User sees blank page for 3-5 seconds on slow networks.
```

**Proposed (progressive):**
```
mount("#app") immediately with AppShell skeleton
→ fetch data in background
→ render content when ready
```

```typescript
// src/main.ts — Progressive Bootstrap

// 1. Mount app IMMEDIATELY with loading state
const app = createApp(App);
app.use(pinia).use(router).use(i18n);
app.mount("#app");  // ← Mount first, fetch later

// 2. Bootstrap data in background
const bootstrapPromise = bootstrapApp();

async function bootstrapApp() {
  try {
    // Phase 1: Auth (critical path)
    const currentUser = await useAuthStore().fetchCurrentUser();
    
    // Phase 2: Parallel non-critical fetches
    const promises = [useActuatorV1Store().fetchServerInfo()];
    if (currentUser) {
      promises.push(
        useSubscriptionV1Store().fetchSubscription(),
        useWorkspaceV1Store().fetchWorkspaceList(),
      );
    }
    await Promise.all(promises);
    
    // Phase 3: Signal ready
    useAppReadyStore().setReady(true);
  } catch (error) {
    console.error("[Bootstrap] Failed:", error);
    useAppReadyStore().setBootstrapError(error);
  }
}
```

```vue
<!-- src/App.vue — Conditional rendering -->
<template>
  <template v-if="appReady">
    <AuthContext>
      <router-view />
    </AuthContext>
  </template>
  <template v-else-if="bootstrapError">
    <BootstrapErrorPage :error="bootstrapError" @retry="retryBootstrap" />
  </template>
  <template v-else>
    <AppShellSkeleton />  <!-- Branded loading skeleton -->
  </template>
</template>
```

```vue
<!-- src/components/AppShellSkeleton.vue — Loading state -->
<template>
  <div class="h-screen flex items-center justify-center bg-background">
    <div class="flex flex-col items-center gap-4">
      <img src="/logo.svg" alt="Bytebase" class="w-12 h-12 animate-pulse" />
      <div class="w-48 h-1 bg-muted rounded overflow-hidden">
        <div class="h-full bg-accent animate-progress-bar" />
      </div>
    </div>
  </div>
</template>
```

### 2.2 Merge i18n — Shared Namespace (Fix BUG 2.4)

**Strategy**: Keep `vue-i18n` as primary. React's `i18next` loads from same source files.

```typescript
// src/react/i18n.ts — Load from Vue locale files

import enUS from "@/locales/en-US.json";
import zhCN from "@/locales/zh-CN.json";
// ... other locales

i18next.init({
  resources: {
    "en-US": { translation: enUS },  // ← Same source as vue-i18n
    "zh-CN": { translation: zhCN },
  },
  // React-specific translations added as secondary namespace
  ns: ["translation", "react"],
  defaultNS: "translation",
});
```

```diff
// Delete or deprecate:
-src/react/locales/en.json   (130KB — mostly duplicated)
-src/react/locales/zh.json
-src/react/locales/ja.json
-src/react/locales/es.json
-src/react/locales/vi.json

// Keep only React-specific keys in:
+src/react/locales/react-en.json  (~10KB — React-only UI strings)
```

**Estimated savings**: ~500KB raw, ~100KB gzipped.

### 2.3 Strip Console.debug in Production (Fix BUG 2.6)

```typescript
// vite.config.ts — Add esbuild drop

export default defineConfig({
  esbuild: {
    drop: mode === "production" ? ["console", "debugger"] : [],
    // OR more targeted:
    pure: mode === "production" ? ["console.debug", "console.log"] : [],
  },
});
```

**Alternative (preserves console.error/warn):**
```typescript
esbuild: {
  pure: mode === "production" ? ["console.debug"] : [],
},
```

### 2.4 Optimize Route Watcher (Fix BUG 2.3)

```diff
// src/App.vue — Replace lodash deep comparison with shallow

-import { cloneDeep, isEqual } from "lodash-es";
+const PRESERVED_FIELDS = ["mode", "customTheme", "lang", "project", "filter"] as const;

 watch(route, (current, prev) => {
-  const preservedQuery = cloneDeep(current.query);
-  for (const field of fields) {
-    if (!(field in current.query) && field in prev.query) {
-      preservedQuery[field] = prev.query[field];
-    }
-  }
-  if (isEqual(current.query, preservedQuery)) return;
-  router.replace({ ...current, query: preservedQuery });

+  // Shallow check — only examine known fields
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

### 2.5 True Lazy Monaco (Fix BUG 2.5)

```diff
// src/router/sqlEditor.ts — Don't preload Monaco chunk

 {
   path: "/sql-editor",
   component: () => import("@/layouts/SQLEditorLayout.vue"),
-  // Monaco is preloaded when SQLEditorLayout mounts
+  // Monaco loads on-demand when editor tab is opened
 }

// src/views/sql-editor/EditorPanel/index.vue
-import { editor } from "monaco-editor"; // ← Static import pulls entire chunk
+const MonacoEditor = defineAsyncComponent(() =>
+  import("@/components/MonacoEditor/MonacoEditor.vue")
+);
```

---

## 3. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | Progressive bootstrap + AppShellSkeleton | MEDIUM | 4h |
| 2 | Strip console.debug via esbuild | LOW | 0.5h |
| 3 | Optimize route watcher (remove lodash) | LOW | 1h |
| 4 | Merge i18n shared namespace | MEDIUM | 6h |
| 5 | True lazy Monaco loading | LOW | 2h |

**Total**: ~13.5h (~2 days)

---

## 4. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Time to first paint | 3-5s (blank page) | <500ms (skeleton) |
| i18n bundle size | ~1.2MB raw | ~700KB raw (-40%) |
| Console.debug in production | Active | Stripped |
| Route watcher CPU (per nav) | cloneDeep + isEqual | Shallow field check |
| Monaco initial load impact | Preloaded on SQL Editor layout | On-demand per tab |
