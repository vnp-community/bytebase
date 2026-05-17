# TASK-L-002: Idempotency-Aware Auth Retry

> **Source**: SOL-LIM-003 §2.2 | **Priority**: P1 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/connect/middlewares/authInterceptorMiddleware.ts`

## What
Thêm `isSafeToRetry()` helper. Chỉ retry read requests (`Get*`, `List*`, `Search*`, `Batch*`, `Check*`) sau token refresh. Mutations không auto-retry.

## Implementation

```diff
+const SAFE_TO_RETRY_PREFIXES = ["Get", "List", "Search", "Batch", "Check"];
+
+function isSafeToRetry(methodName: string): boolean {
+  return SAFE_TO_RETRY_PREFIXES.some((prefix) => methodName.startsWith(prefix));
+}

 // Inside catch block after successful refreshTokens():
-try {
-  return await next(req);
-} catch (retryError) { ... }
+if (isSafeToRetry(req.method.name)) {
+  try {
+    return await next(req);
+  } catch (retryError) {
+    if (retryError instanceof ConnectError && retryError.code === Code.Unauthenticated) {
+      handleUnauthenticatedFailure({ silent, isLoggedIn });
+    }
+    throw retryError;
+  }
+}
+// Mutations: refresh succeeded but don't retry — throw original error
+throw error;
```

## AC
- [ ] `isSafeToRetry("GetDatabase")` returns `true`
- [ ] `isSafeToRetry("CreateDatabase")` returns `false`
- [ ] After token refresh: read requests retried, mutations throw to caller
- [ ] No behavioral change for Login/Signup/Refresh (still skipped)
