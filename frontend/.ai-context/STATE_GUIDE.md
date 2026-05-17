# State Management Guide

> **TL;DR**: Server data → useVueState(Pinia) or TanStack Query hook. Client state → Zustand.

---

## Decision Tree

```
Is the data from the backend API (database, project, user, issue, etc.)?
  │
  ├─ YES → "Server State"
  │   │
  │   ├─ Does a TanStack Query hook exist? (src/react/hooks/queries/)
  │   │   ├─ YES → Use the TanStack Query hook (PREFERRED)
  │   │   │         e.g. const { data } = useDatabase(name);
  │   │   │
  │   │   └─ NO → Use useVueState + Pinia store (LEGACY)
  │   │            e.g. const databases = useVueState(() => useDatabaseV1Store().databaseList);
  │   │            ⚠️ DO NOT create new useVueState calls if a query hook exists
  │   │
  │   └─ For mutations (create/update/delete):
  │       ├─ TanStack Query: useMutation() from query hook
  │       └─ Legacy: call ConnectRPC client directly + invalidate Pinia cache
  │
  └─ NO → "Client State"
      │
      ├─ Auth (currentUser, isLoggedIn) → useAuthStore() (Zustand)
      │   Legacy: useVueState(() => useAuthStore().currentUser)
      │
      ├─ UI Preferences (locale, theme, sidebar) → useUIStore() (Zustand)
      │
      ├─ SQL Editor tabs/session → useSQLEditorStore() (Zustand)
      │
      └─ Component-local → useState() / useReducer()
```

---

## State Layer Mapping

| Data Type | Current Source | Target Source | Example |
|---|---|---|---|
| Database entity | `useDatabaseV1Store` (Pinia) | `useDatabase()` (TanStack Query) | `useVueState(() => store.getByName(name))` |
| Project entity | `useProjectV1Store` (Pinia) | `useProject()` | |
| Instance entity | `useInstanceV1Store` (Pinia) | `useInstance()` | |
| User entity | `useUserStore` (Pinia) | `useUser()` | |
| Issue / Plan / Rollout | Pinia stores | TanStack Query hooks | |
| Current user | `useAuthStore` (Pinia) | `useAuthStore` (Zustand) | `useVueState(() => useAuthStore().currentUser)` |
| Subscription/Feature | `useSubscriptionV1Store` | TanStack Query | `hasFeature(PlanFeature.FEATURE_X)` |
| Setting (workspace config) | `useSettingV1Store` | TanStack Query | |
| SQL Editor tabs | Custom Pinia store | `useSQLEditorStore` (Zustand) | |
| Locale/Theme | localStorage + Pinia | `useUIStore` (Zustand) | |

---

## Pinia Stores → File Map

| Store | File | Domain |
|---|---|---|
| `useDatabaseV1Store` | `src/store/modules/v1/database.ts` | Database CRUD + cache |
| `useProjectV1Store` | `src/store/modules/v1/project.ts` | Project CRUD + IAM |
| `useInstanceV1Store` | `src/store/modules/v1/instance.ts` | Instance CRUD |
| `useAuthStore` | `src/store/modules/v1/auth.ts` | Login, logout, currentUser |
| `useSettingV1Store` | `src/store/modules/v1/setting.ts` | Workspace settings |
| `useSubscriptionV1Store` | `src/store/modules/v1/subscription.ts` | Plan features |
| `useIssueV1Store` | `src/store/modules/v1/issue.ts` | Issue listing |
| `usePlanStore` | `src/store/modules/v1/plan.ts` | Plan management |
| `useGroupStore` | `src/store/modules/v1/group.ts` | User groups |
| `usePermissionStore` | `src/store/modules/v1/permission.ts` | RBAC checks |

---

## Common Patterns

### Reading server data in React (LEGACY)

```typescript
import { useVueState } from "@/react/hooks/useVueState";
import { useDatabaseV1Store } from "@/store";

function MyComponent({ dbName }: Props) {
  // useVueState subscribes to Vue reactivity and re-renders on change
  const database = useVueState(() =>
    useDatabaseV1Store().getDatabaseByName(dbName)
  );
  // database is reactive — component re-renders when store updates
}
```

### Reading server data in React (PREFERRED — TanStack Query)

```typescript
import { useDatabase } from "@/react/hooks/queries";

function MyComponent({ dbName }: Props) {
  const { data: database, isLoading, error } = useDatabase(dbName);
  if (isLoading) return <Spinner />;
  if (error) return <ErrorAlert error={error} />;
  // database is type-safe, cached, auto-refetched
}
```

### Mutating data

```typescript
import { useUpdateDatabase } from "@/react/hooks/queries";

function MyComponent() {
  const { mutate: updateDB } = useUpdateDatabase();
  const handleSave = () => {
    updateDB(
      { database: { ...db, labels: newLabels }, updateMask: ["labels"] },
      { onSuccess: () => toast("Saved") }
    );
  };
}
```

---

## Anti-Patterns

| ❌ Don't | ✅ Do |
|---|---|
| Import Pinia store directly in `.tsx` and call `.value` | Use `useVueState()` wrapper or TanStack Query hook |
| Create new `useVueState` calls when a query hook exists | Use the TanStack Query hook |
| Use `useState` for server-fetched data | Use TanStack Query (caching + dedup) |
| Mix Pinia mutations in React components | Use `useMutation` from TanStack Query |
| Access `store.databaseList.value` in React | `useVueState(() => store.databaseList)` (no `.value`) |
