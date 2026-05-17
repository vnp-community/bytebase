# TASK-L-006: Refined ConnectError Filter

> **Source**: SOL-LIM-004 §2.3 | **Priority**: P1 | **Effort**: 1h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/App.vue` (onErrorCaptured handler)

## What
Thu hẹp ConnectError swallow filter từ "all known codes" xuống chỉ 3 codes có explicit handler. Các gRPC errors khác (INTERNAL, DATA_LOSS, UNAVAILABLE) sẽ hiển thị notification.

## Implementation

```diff
-if (error instanceof ConnectError && Object.values(Code).includes(error.code)) {
-  return; // Already handled by interceptor
-}
+// Only these codes are explicitly handled by interceptors
+const INTERCEPTOR_HANDLED_CODES = [
+  Code.Unauthenticated,   // authInterceptor → SessionExpiredSurface
+  Code.PermissionDenied,   // authInterceptor → 403 page
+  Code.NotFound,           // errorNotificationInterceptor → ignored
+];
+
+if (
+  error instanceof ConnectError &&
+  INTERCEPTOR_HANDLED_CODES.includes(error.code)
+) {
+  return; // Handled by specific interceptor logic
+}
+
+// All other errors (INTERNAL, DATA_LOSS, UNAVAILABLE, etc.)
+// → show CRITICAL notification
```

## AC
- [ ] `Code.Unauthenticated` errors still silently handled (no change)
- [ ] `Code.Internal` errors now show CRITICAL notification (was silent)
- [ ] `Code.DataLoss` errors now show CRITICAL notification (was silent)
- [ ] `INTERCEPTOR_HANDLED_CODES` is explicitly documented array (not dynamic)
