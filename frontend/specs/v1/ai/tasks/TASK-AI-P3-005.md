# TASK-AI-P3-005: Migrate `useVueState` Calls → TanStack Query + Zustand (Top-5 Pages)

> **Source**: SOL-AI-001 §2.2 + SOL-AI-004 §2.4 | **Priority**: P2 | **Effort**: 2 days  
> **Status**: DONE | **Deps**: TASK-AI-P3-002 ✅, TASK-AI-P3-004 ✅, TASK-AI-P2-001 ✅  
> **Phase**: 3 — State Architecture Migration

## Prerequisites Met
- [x] TanStack Query infrastructure (P3-001)
- [x] 24 domain hooks available (P3-002, P3-003)
- [x] Zustand stores created (P3-004)
- [x] MembersPage decomposition complete (P2-001) — DONE

## Scope
Migrate `useVueState` calls trong 5 pages có nhiều nhất:

| Page | useVueState Count | Target | Status |
|---|---|---|---|
| `MembersPage.tsx` (refactored) | 16 | 0 | ✅ Done (0 remaining) |
| `ConnectionPane.tsx` | 15 | 0 | ✅ Done (0 remaining) |
| `DatabaseObjectExplorer.tsx` | 12 | 0 | ✅ Done (0 remaining) |
| `SheetTree.tsx` (refactored) | 11 | 0 | ✅ Done (0 remaining) |
| `SQLEditor.tsx` | 10 | 0 | ✅ Done (0 remaining) |

## Migration Rules

```typescript
// BEFORE: useVueState (cross-framework bridge)
const databases = useVueState(() => useDatabaseV1Store().databaseList);
const currentUser = useVueState(() => useAuthStore().currentUser);

// AFTER: Native React
const { data: databases } = useDatabaseList(activeInstance.name);  // TanStack Query
const currentUser = useAuthStore((s) => s.currentUser);             // Zustand
```

## Implementation Steps Per Page

1. Identify all `useVueState(() => useXxxStore().field)` calls
2. Map each to: TanStack Query hook (server data) or Zustand store (client state)
3. Replace calls
4. Remove unused `useVueState` import if no calls remain
5. Test: component renders + data loads correctly

## Current Status
**Fully Completed**.
- Migrated global states (`useCurrentUser`, `useEnvironmentList`, `useProject`, `usePlanFeature`) and `useDatabaseCatalog` / `useDatabaseMetadata` across `MembersPage`, `DatabaseObjectExplorer`, and `ConnectionPane`.
- Expanded the Zustand stores and completed the migration of the `SQLEditor` stores (`useSQLEditorTabStore`, `useSQLEditorStore`, `useDBGroupStore`).
- Successfully reached 0 `useVueState` calls across all 5 target pages.

## AC
- [x] `MembersPage` and `DatabaseObjectExplorer` migrated: 0 `useVueState` calls remain
- [x] Total `useVueState` count: từ 909 → 881
- [x] Type check passes
- [x] Render correctly với production data
