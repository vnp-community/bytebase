# Framework Map — Vue vs React File Ownership

> **Rule**: ALL new features → React. Vue code is legacy, being migrated.

---

## File Ownership Rules

| Signal | Framework | Import Pattern |
|---|---|---|
| File in `src/views/` | **Vue** | `import { ... } from "vue"` |
| File in `src/components/` (no react prefix) | **Vue** | `<script setup lang="ts">` |
| File in `src/layouts/` | **Vue** | Vue single-file component |
| File extension `.vue` | **Vue** | — |
| File in `src/react/` | **React** | `import { useState } from "react"` |
| File extension `.tsx` | **React** | — |
| File in `src/store/` | **Vue (Pinia)** | `import { defineStore } from "pinia"` |
| File in `src/react/stores/` | **React (Zustand)** | `import { create } from "zustand"` |
| File in `src/connect/` | **Framework-agnostic** | ConnectRPC clients |
| File in `src/router/` | **Vue Router** | `import { ... } from "vue-router"` |

---

## Bridge Entry Points (ONLY these files may cross the boundary)

| File | Role | Direction |
|---|---|---|
| `src/react/mount.ts` | Lazy-loads React pages into Vue containers | Vue → React |
| `src/react/ReactPageMount.vue` | Vue route component that calls `mountReactPage()` | Vue → React |
| `src/react/hooks/useVueState.ts` | Subscribes React components to Vue reactive state | React → Vue |

### `useVueState` — How the bridge works

```typescript
// useVueState creates a Vue `watch()` subscription and pipes value changes
// into React via useSyncExternalStore.
//
// Usage (LEGACY — prefer TanStack Query for new code):
const databases = useVueState(() => useDatabaseV1Store().databaseList);
const currentUser = useVueState(() => useAuthStore().currentUser);

// Options:
// { deep: true } — track nested field mutations (needed for in-place Object.assign)
const tabs = useVueState(() => tabStore.openTabList, { deep: true });
```

> ⚠️ **DO NOT create new `useVueState` calls.** Use TanStack Query hooks or Zustand stores instead.

---

## Mount System — How React Pages Load

```
1. Vue Router matches route → component: ReactPageMount.vue
2. ReactPageMount.vue reads `props.page` (e.g. "MembersPage")
3. Calls mountReactPage(container, "MembersPage")
4. mount.ts resolves "./pages/settings/MembersPage.tsx" via import.meta.glob
5. Loads React, ReactDOM, i18next
6. createRoot(container).render(<StrictMode><I18nextProvider><MembersPage /></I18nextProvider></StrictMode>)
```

### Glob Patterns in `mount.ts`

```typescript
// These directories are auto-resolved by Vite:
"./pages/settings/*.tsx"                    // Settings pages
"./pages/project/*.tsx"                     // Project pages
"./pages/workspace/*.tsx"                   // Workspace pages
"./pages/auth/*.tsx"                        // Auth pages
"./plugins/agent/components/AgentWindow.tsx" // Agent plugin
"./components/auth/SessionExpiredSurface.tsx" + "InactiveRemindModal.tsx"
"./components/sql-editor/*.tsx"             // SQL Editor components
"./components/*.tsx"                        // Shared components (excluding .test.tsx)
```

If your new page is in one of these directories, it will be auto-discovered.
If it's in a **new** directory, add a glob pattern to `mount.ts`.

---

## Migration Status

| Area | Status | Notes |
|---|---|---|
| Settings pages | ✅ Migrated to React | 53 .tsx files |
| Project pages | ✅ Migrated to React | 117 .tsx files |
| Auth pages | ✅ Migrated to React | 18 .tsx files |
| Workspace pages | 🔄 Partial | 3 .tsx files |
| SQL Editor | 🔄 Hybrid | Vue shell + React components |
| Layouts | ❌ Still Vue | `DashboardLayout.vue`, `SplashLayout.vue` |
| Root App Shell | ❌ Still Vue | `App.vue` → Vue Router → ReactPageMount |
| Store (State) | 🔄 Hybrid | Pinia (Vue) + Zustand (React) + useVueState bridge |
| i18n | 🔄 Dual | vue-i18n (`.vue`) + i18next (`.tsx`) |

**Target**: Phase-by-phase migration to React-only SPA. See `specs/v1/ai/solutions/SOL-AI-001`.
