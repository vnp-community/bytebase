# TASK-W-003: Token Refresh Resilience

> **Source**: SOL-WEAK-003 §3.4-3.5 | **Priority**: P1 | **Effort**: 2.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/connect/refreshToken.ts`

## What
1. Broadcast "failed" message when token refresh fails (currently only broadcasts "complete")
2. Add Web Locks API fallback for older browsers

## Implementation

### Change 1: Broadcast failure (~L41-56)
```diff
+type RefreshMessage = "complete" | "failed";

 async function tryAcquireAndRefresh(): Promise<boolean> {
   return navigator.locks.request(LOCK_NAME, { ifAvailable: true }, async (lock) => {
     if (!lock) return false;
+    const channel = new BroadcastChannel(CHANNEL_NAME);
     try {
       await authServiceClientConnect.refresh({});
-      const channel = new BroadcastChannel(CHANNEL_NAME);
-      channel.postMessage("complete");
-      channel.close();
+      channel.postMessage("complete" satisfies RefreshMessage);
       return true;
     } catch (error) {
-      // (implicit: no broadcast on failure)
+      console.error("[TokenRefresh] Refresh failed:", error);
+      channel.postMessage("failed" satisfies RefreshMessage);
       return false;
+    } finally {
+      channel.close();
     }
   });
 }
```

### Change 2: Handle failure message in waiting tabs
```diff
 channel.onmessage = (event) => {
   clearTimeout(timeout);
   channel.close();
-  resolve(true);
+  resolve(event.data === "complete");
 };
```

### Change 3: Web Locks fallback
```diff
+let refreshMutex = Promise.resolve();

 async function refreshTokens(): Promise<void> {
+  if (typeof navigator.locks?.request !== "function") {
+    console.warn("[TokenRefresh] Web Locks unavailable, using fallback");
+    refreshMutex = refreshMutex.then(() => authServiceClientConnect.refresh({}));
+    await refreshMutex;
+    return;
+  }
   // ... existing Web Locks implementation
 }
```

## AC
- [ ] Failed refresh broadcasts "failed" to other tabs
- [ ] Waiting tabs resolve immediately on "failed" (not wait 10s timeout)
- [ ] When `navigator.locks` is undefined, fallback mutex prevents crash
- [ ] All refresh errors logged to console.error
