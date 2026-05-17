# SOL-AI-004 — Unified State Management Via TanStack Query + Zustand

> **Resolves**: ISS-AI-004 (Topology State Management Đa Tầng)  
> **Type**: Architectural Change (State Layer)  
> **Priority**: High  
> **Effort**: Large (2–3 months)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Thay thế 4-layer state topology (Pinia + Zustand + Custom Cache + Vue Reactivity Bridge) bằng **2-layer unified approach** dễ hiểu cho AI:

```
Before (4 layers):  Pinia → Cache → useVueState → React
After  (2 layers):  TanStack Query (server state) + Zustand (client state)
```

---

## 2. Giải Pháp Kiến Trúc

### 2.1 State Classification Rõ Ràng

```
Server State (từ API):      → TanStack Query
  Database, Project, Instance, Issue, Plan, Rollout,
  User, Role, Policy, Setting, Subscription, Audit

Client State (UI-only):     → Zustand
  Auth (currentUser, isLoggedIn)
  UI Preferences (theme, locale, panel sizes)
  SQL Editor tabs (session state)
  Notifications
  Quickstart progress
```

### 2.2 TanStack Query Integration

**Cài đặt và setup:**

```typescript
// src/react/providers/QueryProvider.tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,    // 5 min — replaces Pinia entity cache TTL
      gcTime: 30 * 60 * 1000,      // 30 min garbage collection
      retry: (failureCount, error) => {
        if (isConnectError(error, Code.NotFound)) return false;
        return failureCount < 3;
      },
    },
  },
});
```

**Query hooks per domain:**

```typescript
// src/react/hooks/queries/useDatabase.ts
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { databaseServiceClientConnect } from "@/connect";

// SINGLE source of truth — AI always uses THIS, not Pinia store
export function useDatabase(name: string) {
  return useQuery({
    queryKey: ["database", name],
    queryFn: () => databaseServiceClientConnect.getDatabase({ name }),
    enabled: !!name,
  });
}

export function useDatabaseList(parent: string) {
  return useQuery({
    queryKey: ["databases", parent],
    queryFn: () => databaseServiceClientConnect.listDatabases({ parent }),
  });
}

export function useUpdateDatabase() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ database, updateMask }: UpdateDatabaseParams) =>
      databaseServiceClientConnect.updateDatabase({ database, updateMask }),
    onSuccess: (updated) => {
      queryClient.setQueryData(["database", updated.name], updated);
    },
  });
}
```

**Query key conventions (AI-memorizable):**

```typescript
// src/react/hooks/queries/query-keys.ts
export const queryKeys = {
  database: {
    all: ["databases"] as const,
    list: (parent: string) => ["databases", parent] as const,
    detail: (name: string) => ["database", name] as const,
  },
  project: {
    all: ["projects"] as const,
    list: () => ["projects"] as const,
    detail: (name: string) => ["project", name] as const,
  },
  // ... 28 more domains
} as const;
```

### 2.3 Zustand Simplification

Giảm từ 4 state sources xuống **1 Zustand store per domain** (client state only):

```typescript
// src/react/stores/auth.ts — Client state only
export const useAuthStore = create<AuthStore>()(
  devtools(
    persist(
      (set) => ({
        currentUser: null as User | null,
        isLoggedIn: false,
        requireResetPassword: false,
        // Actions
        setCurrentUser: (user: User) => set({ currentUser: user, isLoggedIn: true }),
        logout: () => set({ currentUser: null, isLoggedIn: false }),
      }),
      { name: "bb-auth-store" }
    )
  )
);

// src/react/stores/ui.ts — UI preferences only
export const useUIStore = create<UIStore>()(
  persist(
    (set) => ({
      locale: "en-US",
      theme: "light",
      sidebarCollapsed: false,
      setLocale: (locale: string) => set({ locale }),
      setTheme: (theme: "light" | "dark") => set({ theme }),
    }),
    { name: "bb-ui-store" }
  )
);
```

### 2.4 Migration Map (Pinia → TanStack Query)

| Pinia Store | TanStack Query Hook | Notes |
|---|---|---|
| `useDatabaseV1Store.getOrFetchDatabaseByName` | `useDatabase(name)` | Direct replacement |
| `useDatabaseV1Store.fetchDatabaseList` | `useDatabaseList(parent)` | |
| `useProjectV1Store.getOrFetchProjectByName` | `useProject(name)` | |
| `useInstanceV1Store.fetchInstanceList` | `useInstanceList()` | |
| `useIssueV1Store.fetchIssueList` | `useIssueList(parent)` | |
| `usePolicyV1Store.getOrFetchPolicy` | `usePolicy(name)` | |
| `useSettingV1Store.fetchSetting` | `useSetting(name)` | |
| All update mutations | `useUpdate{Domain}()` | With cache invalidation |

### 2.5 Phased Migration Timeline

```
Month 1: Setup TanStack Query infrastructure + 5 core domains
  - QueryProvider wrapper
  - useDatabase, useProject, useInstance, useUser, useEnvironment

Month 2: Migrate remaining 25+ domain queries
  - Issue, Plan, Rollout, Policy, Setting, Subscription...

Month 3: Remove Pinia stores (after all consumers migrated)
  - Delete src/store/modules/v1/*.ts (33 files)
  - Delete src/store/cache.ts
  - Remove useVueState calls (now replaced by query hooks)
```

### 2.6 State Flow Sau Migration (AI-friendly)

```
React Component
  ↓ useDatabase("instances/prod/databases/mydb")  ← ONE line
    ↓ TanStack Query: check cache
      hit  → return cached data immediately
      miss → databaseServiceClientConnect.getDatabase(...)
               ↓ ConnectRPC → Backend
               ↓ Cache result
    ↓ { data: Database, isLoading, error }  ← Type-safe return
  Component renders
```

**2 levels deep** — vs 8 levels hiện tại. AI dễ trace và debug.

---

## 3. Thay Đổi Technical Design Document

**Cập nhật `specs/technical-design-document.md` Section 3.3 "State Management Design":**

Thay toàn bộ Pinia pattern bằng:
- **Server State**: TanStack Query patterns + query-keys.ts conventions
- **Client State**: Zustand stores (auth, ui, sqlEditor only)
- **Deprecated**: Pinia, custom cache.ts, useVueState bridge

---

## 4. Implementation Checklist

- [ ] Install `@tanstack/react-query` v5
- [ ] Tạo `src/react/providers/QueryProvider.tsx`
- [ ] Tạo `src/react/hooks/queries/query-keys.ts`
- [ ] Migrate 5 core domain hooks (database, project, instance, user, environment)
- [ ] Migrate remaining 25+ domain hooks
- [ ] Migrate Pinia client state → Zustand stores (auth, ui)
- [ ] Migrate SQL Editor state → dedicated Zustand store
- [ ] Remove Pinia dependencies after full migration
- [ ] Delete `src/store/cache.ts` when unused

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| State layers | 4 | 2 (TanStack Query + Zustand) |
| Data flow depth | 8 levels | 2 levels |
| `useVueState` calls | 68 | 0 |
| State code to understand | ~15,000 LOC | ~3,000 LOC |
| AI state reasoning accuracy | ~40% | > 85% |
