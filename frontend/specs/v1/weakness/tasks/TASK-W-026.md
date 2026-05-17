# TASK-W-026: Progressive Bootstrap

> **Source**: SOL-WEAK-006 §2.1 | **Priority**: P3 | **Effort**: 3.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/main.ts` — mount first, fetch later
- **NEW** `src/components/AppShellSkeleton.vue` — loading skeleton
- **EDIT** `src/App.vue` — conditional render (skeleton / error / app)

## What
Mount app immediately with branded skeleton, fetch auth/config in background, render router-view when ready.

## Implementation — see SOL-WEAK-006 §2.1
1. `main.ts`: mount app before `fetchCurrentUser`, use `bootstrapApp()` async
2. `AppShellSkeleton.vue`: centered logo + progress bar animation
3. `App.vue`: `v-if="appReady"` for AuthContext, `v-else` for skeleton, error page for bootstrap failure

## AC
- [x] App mounts immediately (no blank page)
- [x] Skeleton shown during data fetch
- [x] Bootstrap error shows retry page
- [x] Auth/config data available before router-view renders
