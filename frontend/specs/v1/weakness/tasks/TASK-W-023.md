# TASK-W-023: Lazy Store Reset

> **Source**: SOL-WEAK-008 §2.3 | **Priority**: P3 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: W-021, W-002

## Scope
- **EDIT** `src/router/index.ts` — remove store resets on auth page visit
- **EDIT** `src/store/modules/v1/auth.ts` — move resets into `logout()`

## Implementation — see SOL-WEAK-008 §2.3 diff
```diff
// router/index.ts — REMOVE:
-if (isAuthRelatedRoute(to.name)) {
-  useDatabaseV1Store().reset();
-  useProjectV1Store().reset();
-  useInstanceV1Store().reset();
-  next(); return;
-}

// auth.ts — ADD to logout() after retry logic:
+useDatabaseV1Store().reset();
+useProjectV1Store().reset();
+useInstanceV1Store().reset();
+useEnvironmentV1Store().reset();
```

## AC
- [x] Visiting auth page does NOT clear caches
- [x] Browser back from `/auth/signin` preserves data
- [x] Only `logout()` resets domain stores
