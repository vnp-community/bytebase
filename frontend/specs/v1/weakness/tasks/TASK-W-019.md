# TASK-W-019: NotFound Default Notification

> **Source**: SOL-WEAK-005 §2.2 | **Priority**: P3 | **Effort**: 1h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/connect/middlewares/errorNotificationMiddleware.ts` (~L37-42)

## What
Remove `Code.NotFound` from default ignored codes so missing resources show notification.

## Implementation
```diff
 if ((ignoredCodes.length === 0
-  ? [Code.NotFound, Code.Unauthenticated]
+  ? [Code.Unauthenticated]
   : ignoredCodes
 ).includes(error.code)) {
```

Note: Callers using `getOrCreate` patterns should explicitly pass `ignoredCodes: [Code.NotFound]`.

## AC
- [x] NotFound errors show notification by default
- [x] Unauthenticated still suppressed
- [x] Callers with explicit `ignoredCodes` unaffected
