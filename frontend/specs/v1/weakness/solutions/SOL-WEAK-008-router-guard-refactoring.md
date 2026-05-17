# SOL-WEAK-008: Router Guard Refactoring — Registry-Based, Non-Destructive, Unified Title

> **Source**: [BUG-WEAK-008](../bugs/BUG-WEAK-008-router-guard-complexity.md)  
> **Severity**: MEDIUM → **Target**: RESOLVED  
> **Status**: PROPOSED | **Created**: 2026-05-13

---

## 1. Tóm tắt

Refactor 9+ branch router guard thành pipeline of composable guard functions, registry-based whitelist, lazy store reset (only on logout), và unified title manager.

---

## 2. Thiết kế Chi tiết

### 2.1 Guard Pipeline Architecture

```typescript
// src/router/guards/index.ts

import type { RouteLocationNormalized, NavigationGuardNext } from "vue-router";

type GuardResult = 
  | { action: "next" }
  | { action: "redirect"; to: string | RouteLocationNormalized }
  | { action: "continue" };  // Pass to next guard in pipeline

type GuardFn = (
  to: RouteLocationNormalized,
  from: RouteLocationNormalized
) => GuardResult | Promise<GuardResult>;

/**
 * Execute guards in order. First guard that returns "next" or "redirect" wins.
 * Guards returning "continue" pass control to the next guard.
 */
async function executeGuardPipeline(
  guards: GuardFn[],
  to: RouteLocationNormalized,
  from: RouteLocationNormalized,
  next: NavigationGuardNext
): Promise<void> {
  for (const guard of guards) {
    const result = await guard(to, from);
    switch (result.action) {
      case "next":
        next();
        return;
      case "redirect":
        next(result.to);
        return;
      case "continue":
        break; // Try next guard
    }
  }
  // Fallback: no guard handled → 404
  console.warn("[Router] No guard handled route:", to.fullPath);
  next({ name: "404" });
}

// Pipeline definition (order matters):
const guardPipeline: GuardFn[] = [
  infiniteLoopGuard,
  errorPageBypassGuard,
  oauthCallbackGuard,
  oauthConsentGuard,     // ← NEW: with auth check (Fix BUG 2.3)
  authRedirectGuard,
  loginEnforcementGuard,
  mfaEnforcementGuard,
  passwordResetGuard,
  routeRegistryGuard,    // ← NEW: registry-based (Fix BUG 2.1)
];
```

### 2.2 Registry-Based Route Whitelist (Fix BUG 2.1)

```typescript
// src/router/guards/route-registry.ts

const allowedRoutes = new Set<string>();

/**
 * Routes register themselves as allowed.
 * Called during route definition — no more hardcoded string list.
 */
export function registerAllowedRoute(routeName: string): void {
  allowedRoutes.add(routeName);
}

/** Check if a route is in the registry */
export function isRouteAllowed(routeName: string): boolean {
  return allowedRoutes.has(routeName);
}

// Usage in route definitions:
// src/router/dashboard/workspace.ts
const routes = [
  {
    name: "workspace.database",
    path: "/databases",
    component: () => import("..."),
    meta: { registered: true }, // ← marker
  },
];

// Auto-register all routes with `registered: true` meta
router.getRoutes().forEach(route => {
  if (route.meta.registered) {
    registerAllowedRoute(route.name as string);
  }
});

// Guard implementation:
const routeRegistryGuard: GuardFn = (to) => {
  if (!to.name || isRouteAllowed(to.name as string)) {
    return { action: "next" };
  }
  console.warn("[Router] Route not in registry:", to.name, to.fullPath);
  return { action: "redirect", to: { name: "404" } };
};
```

### 2.3 Lazy Store Reset — Only on Logout (Fix BUG 2.2)

```diff
// src/router/index.ts — REMOVE store reset on auth page visit

-if (isAuthRelatedRoute(to.name as string)) {
-  useDatabaseV1Store().reset();
-  useProjectV1Store().reset();
-  useInstanceV1Store().reset();
-  // ...
-  next(); return;
-}

// src/store/modules/v1/auth.ts — Move resets to logout ONLY
const logout = async () => {
  // ... API call + retry (from SOL-WEAK-003) ...
  
  // Reset domain stores on actual logout
+ useDatabaseV1Store().reset();
+ useProjectV1Store().reset();
+ useInstanceV1Store().reset();
+ useEnvironmentV1Store().reset();
  // ... all domain store resets ...

  // Navigate to signin
  router.push({ name: AUTH_SIGNIN_MODULE });
};
```

**Rationale**: Visiting `/auth/signin` via browser back button should NOT destroy cached data. Only explicit logout should clear state. If user navigates forward again, cached data is still available — no unnecessary re-fetch.

### 2.4 OAuth Consent Auth Guard (Fix BUG 2.3)

```typescript
// src/router/guards/oauth-consent.ts

const oauthConsentGuard: GuardFn = (to) => {
  if (to.name !== OAUTH2_CONSENT_MODULE) {
    return { action: "continue" };
  }
  
  // OAuth consent REQUIRES authenticated user
  const authStore = useAuthStore();
  if (!authStore.isLoggedIn) {
    return {
      action: "redirect",
      to: {
        name: AUTH_SIGNIN_MODULE,
        query: { redirect: to.fullPath },
      },
    };
  }
  
  return { action: "next" };
};
```

### 2.5 Unified Title Manager (Fix BUG 2.4)

```typescript
// src/utils/title-manager.ts

let titleSetBy: "vue" | "react" | null = null;
let titleTimeoutId: ReturnType<typeof setTimeout> | null = null;

/**
 * Set document title with source tracking.
 * React title takes priority over Vue title (set later due to async mount).
 */
export function setPageTitle(title: string, source: "vue" | "react"): void {
  // If React already set title, don't let Vue overwrite
  if (source === "vue" && titleSetBy === "react") {
    return;
  }
  
  // Debounce to prevent rapid flicker
  if (titleTimeoutId) clearTimeout(titleTimeoutId);
  titleTimeoutId = setTimeout(() => {
    document.title = `${title} — Bytebase`;
    titleSetBy = source;
    titleTimeoutId = null;
  }, 50);
}

/** Reset title tracking on navigation start */
export function resetTitleTracking(): void {
  titleSetBy = null;
}
```

```diff
// src/router/index.ts — Use title manager
+import { setPageTitle, resetTitleTracking } from "@/utils/title-manager";

+router.beforeEach(() => {
+  resetTitleTracking();
+});

 router.afterEach((to) => {
   if (to.params.projectId || to.meta.overrideDocumentTitle) return;
   nextTick(() => {
-    if (to.meta.title) { setDocumentTitle(to.meta.title(to)); }
+    if (to.meta.title) { setPageTitle(to.meta.title(to), "vue"); }
   });
 });

// React pages:
// useEffect(() => setPageTitle("Settings", "react"), []);
```

---

## 3. Migration Plan

| Phase | Thay đổi | Risk | Effort |
|-------|----------|------|--------|
| 1 | Implement guard pipeline architecture | MEDIUM | 4h |
| 2 | Registry-based route whitelist | MEDIUM | 3h |
| 3 | Move store resets to logout only | MEDIUM | 2h |
| 4 | OAuth consent auth guard | LOW | 1h |
| 5 | Unified title manager | LOW | 2h |

**Total**: ~12h (1.5 days)

---

## 4. Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Guard branches in beforeEach | 9+ (monolith) | 0 (pipeline of composable guards) |
| Route whitelist maintenance | Manual hardcoded strings | Auto-registered from route definitions |
| Cache destruction on back-navigate | All stores reset | No destruction (only on logout) |
| Title flicker on React pages | Vue→React overwrite | Debounced, source-tracked |
| OAuth consent security | Bypasses all guards | Auth-checked |
