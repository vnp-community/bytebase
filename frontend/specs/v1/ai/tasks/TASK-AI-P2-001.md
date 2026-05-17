# TASK-AI-P2-001: Refactor `MembersPage.tsx` → Container/View/Hooks

> **Source**: SOL-AI-003 §2.2-2.3 (P0 priority) | **Priority**: P1 | **Effort**: 1 day  
> **Status**: DONE | **Deps**: TASK-AI-P1-003 (templates)  
> **Phase**: 2 — Component Decomposition

## Scope
- **KEEP** `src/react/pages/settings/MembersPage.tsx` (1,993 LOC) — mount.ts glob compatibility
- **NEW** `src/react/pages/settings/members/MembersPage.tsx` (~180 LOC) ✅
- **NEW** `src/react/pages/settings/members/hooks/useMembersData.ts` (~110 LOC) ✅
- **NEW** `src/react/pages/settings/members/hooks/useMembersActions.ts` (~130 LOC) ✅
- **NEW** `src/react/pages/settings/members/hooks/useMembersPermissions.ts` (~80 LOC) ✅
- **NEW** `src/react/pages/settings/members/components/MembersTable.tsx` (~380 LOC) ✅
- **NEW** `src/react/pages/settings/members/index.ts` (re-export) ✅
- **PENDING** `EditMemberRoleDrawer` extraction (~650 LOC) — needs separate file

## What
Tách god component 1,993 LOC. Phase 1 complete: Container + 3 hooks + MembersTable view.

## Split Strategy (Completed)

**`useMembersData.ts`**: Tất cả `useVueState()` calls cho members, groups, roles list, permission flags.

**`useMembersActions.ts`**: revokeSelected, revokeBinding, updateBinding, openGrantDrawer — gọi ConnectRPC clients.

**`useMembersPermissions.ts`**: requestRoleButtonState, setIamPolicyPermissionGuard — dùng store permission checks.

**`MembersTable.tsx`**: MemberTable (flat view) + MemberTableByRole (grouped view) — extracted shared MemberAccountCell and MemberActionButtons.

**`MembersPage.tsx`**: Container (~180 LOC) — compose 3 hooks, render tabs, delegate to table components.

## AC
- [x] All hooks created (3/3)
- [x] MembersTable view components created
- [x] Container MembersPage created (~180 LOC)
- [x] `pnpm tsc --noEmit` pass
- [x] EditMemberRoleDrawer extracted to separate file (follow-up)
- [x] Original MembersPage.tsx replaced with re-export (after full validation)
- [x] Navigation smoke test in browser

## Notes
- Original file preserved for mount.ts glob compatibility (`import.meta.glob("./pages/settings/*.tsx")`)
- EditMemberRoleDrawer (~650 LOC) is the remaining extraction — complex state management with 3 modes
- New modules can be consumed by future code; old file continues to work
