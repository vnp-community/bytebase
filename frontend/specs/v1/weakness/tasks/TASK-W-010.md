# TASK-W-010: Sidebar Bridge Migration

> **Source**: SOL-WEAK-001 Phase 6 | **Priority**: P1 | **Effort**: 2h  
> **Status**: DONE | **Deps**: W-007

## Scope
- **EDIT** `src/react/mountSidebar.ts`
- **EDIT** `src/react/mountProjectSidebar.ts`

## What
Apply same `BridgeLifecycleManager` pattern (AbortController, typed props, error handling) to sidebar mount functions. Replace `any` types.

## AC
- [ ] `mountSidebar.ts` uses abort-safe mounting
- [ ] `mountProjectSidebar.ts` uses abort-safe mounting
- [ ] No `any` types remaining in either file
- [ ] Sidebar unmount/remount doesn't leak React roots
