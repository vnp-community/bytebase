# TASK-W-002: Auth Error Transparency & Logout Retry

> **Source**: SOL-WEAK-003 §3.2-3.3 | **Priority**: P1 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/store/modules/v1/auth.ts` (2 changes: `fetchCurrentUser` ~L219, `logout` ~L198)

## What
1. Replace empty catch in `fetchCurrentUser` with discriminated error logging
2. Add 3x retry with backoff to `logout` API call

## Implementation

### Change 1: `fetchCurrentUser` (~L219-227)
```diff
 const fetchCurrentUser = async () => {
   try {
     const user = await userStore.fetchCurrentUser();
     currentUserName.value = user.name;
     return user;
-  } catch {
-    // do nothing.
+  } catch (error) {
+    if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
+      return undefined; // Expected when not logged in
+    }
+    console.error("[AuthStore] fetchCurrentUser failed:", error);
+    return undefined;
   }
 };
```

### Change 2: `logout` (~L198-216)
```diff
 const logout = async () => {
-  try {
-    await authServiceClientConnect.logout({});
-  } catch {
-    // nothing
-  } finally {
+  let logoutSuccess = false;
+  for (let attempt = 0; attempt < 3; attempt++) {
+    try {
+      await authServiceClientConnect.logout({});
+      logoutSuccess = true;
+      break;
+    } catch (error) {
+      console.warn(`[AuthStore] Logout attempt ${attempt + 1} failed:`, error);
+      if (attempt < 2) await new Promise(r => setTimeout(r, 500 * (attempt + 1)));
+    }
+  }
+  if (!logoutSuccess) {
+    console.error("[AuthStore] All logout attempts failed — server session may persist");
+  }
   // ... existing cleanup code continues unchanged
```

## AC
- [ ] Non-401 errors in `fetchCurrentUser` logged to console.error
- [ ] 401 errors silently return undefined (no change in behavior)
- [ ] Logout retries up to 3 times with increasing delay
- [ ] Cleanup proceeds even if all logout attempts fail
