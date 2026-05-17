# Bridge Contract — Vue ↔ React Interop

> The frontend is a **hybrid app**: Vue 3 shell (router, layouts) hosting React 19 page content.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  Vue Shell (App.vue)                                    │
│  ├── Vue Router (src/router/)                           │
│  ├── Layouts (DashboardLayout.vue, SplashLayout.vue)    │
│  ├── Pinia Stores (src/store/modules/v1/)               │
│  │                                                      │
│  │   Route matched → component: ReactPageMount.vue      │
│  │                     ↓ props.page = "MembersPage"     │
│  │   ┌───────────────────────────────────────────┐      │
│  │   │  React Island (mountReactPage)            │      │
│  │   │  ├── StrictMode                           │      │
│  │   │  │   └── I18nextProvider                  │      │
│  │   │  │       └── <MembersPage />              │      │
│  │   │  │           ├── useVueState() → Pinia    │      │
│  │   │  │           ├── ConnectRPC clients        │      │
│  │   │  │           └── Zustand stores           │      │
│  │   └───────────────────────────────────────────┘      │
│  │                                                      │
└─────────────────────────────────────────────────────────┘
```

---

## Bridge File #1: `src/react/mount.ts`

**Purpose**: Lazy-loads a React component by name and mounts it into a DOM container.

```typescript
// Vue calls:
const root = await mountReactPage(container, "MembersPage");
// React calls: (to update props)
await updateReactPage(root, "MembersPage", newProps);
```

**How it resolves names**: Uses `import.meta.glob("./pages/settings/*.tsx")` etc. to build a loader map.
The `page` name MUST match the file's **named export** (not the filename alone).

**Key constraint**: Page must be in a globbed directory (see FRAMEWORK_MAP.md § "Glob Patterns").

---

## Bridge File #2: `src/react/ReactPageMount.vue`

**Purpose**: Vue route component that creates a DOM container and calls `mountReactPage()`.

```vue
<!-- Simplified -->
<template>
  <div ref="container" />
</template>
<script setup>
const props = defineProps<{ page: string }>();
const container = ref<HTMLElement>();
onMounted(() => mountReactPage(container.value!, props.page));
</script>
```

**Used in router definitions like:**
```typescript
{
  path: "members",
  component: () => import("@/react/ReactPageMount.vue"),
  props: { page: "MembersPage" },  // ← name lookup
}
```

---

## Bridge File #3: `src/react/hooks/useVueState.ts`

**Purpose**: Subscribes a React component to Vue reactive state (Pinia store, ref, computed).

**Mechanism**:
1. Creates a Vue `watch()` on the getter
2. Stores the latest value in a `useRef`
3. Uses `useSyncExternalStore` to notify React of changes
4. Also re-evaluates getter on each render to catch closure changes (props)

```typescript
export function useVueState<T>(getter: () => T, options?: { deep?: boolean }): T
```

**When to use `deep: true`**:
- Collection getters where items are mutated in-place via `Object.assign()`
- Example: SQL Editor tab store where tab properties change without resetting references

**Common pitfalls**:
- ❌ `const db = useDatabaseV1Store().getDatabaseByName(name)` — Vue reactive outside React lifecycle
- ✅ `const db = useVueState(() => useDatabaseV1Store().getDatabaseByName(name))` — properly bridged

---

## Cross-Framework Communication

### Vue → React: Props via route

```typescript
// Router (Vue): passes props to React page
props: { page: "SubscriptionPage", onPlanChange: callback }

// React receives them in the component function
export function SubscriptionPage({ onPlanChange }: Props) { ... }
```

### Vue → React: CustomEvent (Shell-level)

```typescript
// Vue shell emits:
window.dispatchEvent(new CustomEvent("bb:locale-change", { detail: "zh-CN" }));

// React listens:
useEffect(() => {
  const handler = (e: CustomEvent) => i18n.changeLanguage(e.detail);
  window.addEventListener("bb:locale-change", handler);
  return () => window.removeEventListener("bb:locale-change", handler);
}, []);
```

### React → Vue: Direct Pinia calls via useVueState

```typescript
// React component can trigger Pinia actions:
const logout = useVueState(() => useAuthStore().logout);
// Then call: logout()
```

---

## Rules for AI

1. **Never import Vue APIs (`ref`, `computed`, `watch`) directly in `.tsx` files** — except inside `useVueState.ts` itself
2. **Never import React APIs (`useState`, `useEffect`) in `.vue` files**
3. **New bridge calls are forbidden** — use TanStack Query or Zustand for new features
4. **`useVueState` is read-only bridge** — it subscribes to Vue state, doesn't create it
5. **Mount lifecycle**: React component mounts AFTER Vue route transition completes — don't assume DOM is ready during Vue `beforeEach`
