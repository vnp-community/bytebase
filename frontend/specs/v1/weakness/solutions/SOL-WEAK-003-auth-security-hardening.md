# SOL-WEAK-003: Auth Security Hardening — Redirect Validation, Token Resilience & Error Transparency

> **Source**: [BUG-WEAK-003](../bugs/BUG-WEAK-003-auth-security-gaps.md)  
> **Severity**: HIGH → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Khắc phục 6 lỗ hổng bảo mật: open redirect, silent auth errors, token refresh race, Web Locks fallback, logout reliability, OAuth listener leak.

---

## 2. Thay đổi Kiến trúc

### Architecture Doc Section 9
- Thêm Redirect Validation Policy
- Cập nhật Token Refresh design (broadcast failure)
- Thêm Web Locks fallback mechanism

### TDD Section 3.5
- Cập nhật Auth Store error handling contract
- Thêm logout retry strategy

---

## 3. Thiết kế Chi tiết

### 3.1 Open Redirect Prevention (Fix BUG 2.3)

```typescript
// src/utils/redirect-validator.ts — NEW

/**
 * Validate redirect URL to prevent open redirect attacks.
 * Only allows relative paths (starts with /, not //).
 */
export function validateRedirectUrl(url: string | undefined | null): string {
  if (!url || typeof url !== "string") return "/";
  
  const trimmed = url.trim();
  
  // Must start with / (relative path)
  if (!trimmed.startsWith("/")) return "/";
  
  // Block protocol-relative URLs (//evil.com)
  if (trimmed.startsWith("//")) return "/";
  
  // Block URLs with protocol (javascript:, data:, etc.)
  if (/^\/[^/]/.test(trimmed) === false) return "/";
  
  // Block backslash (some browsers interpret \/ as //)
  if (trimmed.includes("\\")) return "/";
  
  return trimmed;
}
```

```diff
// src/router/index.ts — Apply validation to redirectParam
+import { validateRedirectUrl } from "@/utils/redirect-validator";

 let redirect = "/";
 if (relayState && typeof relayState === "string") {
-  if (relayState.startsWith("/") && !relayState.startsWith("//")) {
-    redirect = relayState;
-  }
+  redirect = validateRedirectUrl(relayState);
-} else if (redirectParam) {
-  redirect = redirectParam;
+} else if (redirectParam) {
+  redirect = validateRedirectUrl(redirectParam);
 }
```

### 3.2 Auth Error Transparency (Fix BUG 2.1)

```diff
// src/store/modules/v1/auth.ts
 const fetchCurrentUser = async () => {
   try {
     const user = await userStore.fetchCurrentUser();
     currentUserName.value = user.name;
     return user;
-  } catch {
-    // do nothing.
+  } catch (error) {
+    // Log non-401 errors for diagnostics
+    if (error instanceof ConnectError) {
+      if (error.code === Code.Unauthenticated) {
+        // Expected when not logged in — silent
+        return undefined;
+      }
+      console.error("[AuthStore] fetchCurrentUser failed:", error.code, error.message);
+    } else {
+      console.error("[AuthStore] fetchCurrentUser unexpected error:", error);
+    }
+    return undefined;
   }
 };
```

### 3.3 Logout Retry Strategy (Fix BUG 2.2)

```typescript
// src/store/modules/v1/auth.ts

const logout = async () => {
  // Retry logout API up to 2 times before giving up
  let logoutSuccess = false;
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      await authServiceClientConnect.logout({});
      logoutSuccess = true;
      break;
    } catch (error) {
      console.warn(`[AuthStore] Logout attempt ${attempt + 1} failed:`, error);
      if (attempt < 2) {
        await new Promise(r => setTimeout(r, 500 * (attempt + 1)));
      }
    }
  }

  if (!logoutSuccess) {
    console.error("[AuthStore] All logout attempts failed — server session may persist");
    // Proceed with local cleanup anyway to prevent user from being stuck
  }

  // ... existing cleanup (reset stores, clear tokens, etc.)
};
```

### 3.4 Token Refresh — Broadcast Failure (Fix BUG 2.4)

```typescript
// src/connect/refreshToken.ts — Enhanced

type RefreshMessage = "complete" | "failed";

async function tryAcquireAndRefresh(): Promise<boolean> {
  return navigator.locks.request(LOCK_NAME, { ifAvailable: true }, async (lock) => {
    if (!lock) return false;
    
    const channel = new BroadcastChannel(CHANNEL_NAME);
    try {
      await authServiceClientConnect.refresh({});
      channel.postMessage("complete" satisfies RefreshMessage);
      return true;
    } catch (error) {
      console.error("[TokenRefresh] Refresh failed:", error);
      channel.postMessage("failed" satisfies RefreshMessage);  // ← NEW: broadcast failure
      return false;
    } finally {
      channel.close();
    }
  });
}

async function waitForOtherTabRefresh(): Promise<boolean> {
  return new Promise((resolve) => {
    const channel = new BroadcastChannel(CHANNEL_NAME);
    const timeout = setTimeout(() => {
      channel.close();
      resolve(false); // timeout → caller should retry
    }, 10_000);

    channel.onmessage = (event: MessageEvent<RefreshMessage>) => {
      clearTimeout(timeout);
      channel.close();
      if (event.data === "complete") {
        resolve(true);
      } else {
        resolve(false);  // ← NEW: immediate resolution on failure
      }
    };
  });
}
```

### 3.5 Web Locks API Fallback (Fix BUG 2.5)

```typescript
// src/connect/refreshToken.ts — Fallback for missing Web Locks API

let refreshMutex = Promise.resolve();

async function refreshTokens(): Promise<void> {
  if (typeof navigator.locks?.request === "function") {
    // Modern browsers — use Web Locks + BroadcastChannel
    const acquired = await tryAcquireAndRefresh();
    if (!acquired) {
      const otherTabSuccess = await waitForOtherTabRefresh();
      if (!otherTabSuccess) {
        await tryAcquireAndRefresh(); // retry once
      }
    }
  } else {
    // Fallback — simple promise-based mutex (single-tab only)
    console.warn("[TokenRefresh] Web Locks API unavailable, using fallback mutex");
    refreshMutex = refreshMutex.then(async () => {
      try {
        await authServiceClientConnect.refresh({});
      } catch (error) {
        console.error("[TokenRefresh] Fallback refresh failed:", error);
        throw error;
      }
    });
    await refreshMutex;
  }
}
```

### 3.6 OAuth Event Listener Lifecycle (Fix BUG 2.6)

```diff
// src/App.vue

+const handleOAuthUnknown = () => {
+  notificationStore.pushNotification({
+    module: "bytebase",
+    style: "WARN",
+    title: t("oauth.unknown-event"),
+  });
+};

-// Outside lifecycle — never removed
-window.addEventListener("bb.oauth.unknown", () => {
-  notificationStore.pushNotification({ ... });
-});

+onMounted(() => {
+  window.addEventListener("bb.oauth.unknown", handleOAuthUnknown);
+});
+
+onUnmounted(() => {
+  window.removeEventListener("bb.oauth.unknown", handleOAuthUnknown);
+});
```

---

## 4. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | `validateRedirectUrl()` + apply to router | LOW — security fix | 2h |
| 2 | Auth error transparency (console.error) | LOW | 1h |
| 3 | Logout retry strategy | LOW | 1h |
| 4 | Token refresh broadcast failure | MEDIUM | 2h |
| 5 | Web Locks fallback | LOW | 2h |
| 6 | OAuth listener lifecycle fix | LOW | 0.5h |

**Total**: ~8.5h (1 day)

---

## 5. Security Test Cases

- Open redirect: `?redirect=https://evil.com` → must redirect to `/`
- Open redirect: `?redirect=//evil.com` → must redirect to `/`
- Open redirect: `?redirect=javascript:alert(1)` → must redirect to `/`
- Open redirect: `?redirect=/settings` → must redirect to `/settings` ✓
- Logout failure: mock 500 on logout → verify 3 retries, then local cleanup
- Token refresh failure: mock 401 on refresh → verify "failed" broadcast to other tabs
- Web Locks missing: delete `navigator.locks` → verify fallback mutex works

## 6. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Open redirect vectors | 1 (`redirectParam`) | 0 |
| Auth error visibility | 0% (all swallowed) | 100% (console.error) |
| Logout reliability | Fire-and-forget | 3x retry with backoff |
| Tab refresh failure UX | 10s frozen wait | Immediate resolution |
| Browser compat (Web Locks) | Crash on old browsers | Graceful fallback |
