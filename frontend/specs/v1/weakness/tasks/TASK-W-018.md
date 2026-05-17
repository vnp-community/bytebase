# TASK-W-018: Scoped ConnectError Suppression

> **Source**: SOL-WEAK-005 §2.1 | **Priority**: P3 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/App.vue` (`onErrorCaptured` handler, ~L130-148)

## What
Replace blanket ConnectError suppression (all 17 gRPC codes) with explicit list of 3 interceptor-handled codes.

## Implementation — see SOL-WEAK-005 §2.1 diff
```diff
 if (error instanceof ConnectError) {
-  if (Object.values(Code).includes(error.code)) {
-    return;
-  }
+  const INTERCEPTOR_HANDLED_CODES = [Code.Unauthenticated, Code.PermissionDenied, Code.Canceled];
+  if (INTERCEPTOR_HANDLED_CODES.includes(error.code)) return;
+  console.error("[App] Unhandled ConnectError:", error.code, error.message);
 }
-return true;
+return false;
```

## AC
- [x] Only 3 codes suppressed (Unauthenticated, PermissionDenied, Canceled)
- [x] Other ConnectErrors (Internal, DataLoss, etc.) logged and shown as notification
- [x] `return false` allows propagation to Vue error handler
