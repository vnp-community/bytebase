# Navigation Guard Flowchart

> Source: `src/router/index.ts` — `router.beforeEach()`

---

## Guard Decision Flow

```
User navigates to /target-route
  │
  ├─[1] Same route (name + fullPath identical)?
  │     YES → ABORT (prevent infinite loop)
  │
  ├─[2] Error page (/403 or /404)?
  │     YES → ALLOW (error pages always accessible)
  │
  ├─[3] OAuth/OIDC callback route?
  │     YES → ALLOW (auth callbacks bypass all guards)
  │
  ├─[4] OAuth2 consent page?
  │     YES → ALLOW (consent page bypass)
  │
  ├─[5] Logged in AND going to 2FA-setup or password-reset?
  │     YES → ALLOW (these are post-login mandatory steps)
  │
  ├─[6] Logged in AND going to any auth route (signin, etc.)?
  │     YES → REDIRECT to "/" or relay_state/redirect param
  │           (already logged in, no need to sign in again)
  │
  ├─[7] Going to auth route (signin, etc.)?
  │     YES → Reset stores (database, project, instance, conversation)
  │           → ALLOW (let them sign in)
  │
  ├─[8] NOT logged in?
  │     YES → REDIRECT to /auth/signin
  │           (with ?redirect= to return after login)
  │
  ├─[9] 2FA required AND user hasn't set up MFA?
  │     YES → REDIRECT to /auth/2fa-setup
  │
  ├─[10] Password reset required?
  │      YES → REDIRECT to /auth/reset-password
  │
  ├─[11] Route matches allowed patterns?
  │      (environment, instance, project, database, settings, setup, workspace, sql-editor)
  │      YES → ALLOW
  │
  ├─[12] Same path as current? (anchor change)
  │      YES → ALLOW
  │
  └─[13] No match → REDIRECT to /404
```

---

## Debugging Guide

| Symptom | Likely Guard | Check |
|---|---|---|
| Infinite redirect loop | Guard [1] | `from.fullPath === to.fullPath` — may be router push with same params |
| Redirects to /auth/signin on every navigation | Guard [8] | `authStore.isLoggedIn` is false — check if auth bootstrap completed |
| Redirects to /auth/signin after login | Guard [6] | `unauthenticatedOccurred` flag is true — clear it after re-auth |
| 403 on valid route with correct permissions | Guard [11] | Route name doesn't start with an allowed pattern — check route `name` prefix |
| 404 on valid route | Guard [13] | Route not in `allowedRoutePatterns` — add route name prefix |
| Stuck on 2FA setup | Guard [9] | User's `mfaEnabled` is false AND workspace requires MFA |
| Blank page after login | Guard [6] | `relay_state` or `redirect` param has invalid URL — check URL validation |

---

## Allowed Route Name Prefixes

These prefixes pass Guard [11] and are allowed for logged-in users:

```typescript
const allowedRoutePatterns = [
  "environment.dashboard",     // Environment pages
  "workspace.instance",        // Instance pages
  "workspace.project",         // Project pages
  "workspace.database",        // Database pages
  "workspace.setting",         // Settings pages
  "setup",                     // Initial setup wizard
  "workspace",                 // Workspace-level pages
  "sql-editor",                // SQL Editor
];
```

If your new route's `name` doesn't start with one of these prefixes, it will 404.

---

## After-Each Hook

After navigation completes:

```
Route has projectId param OR overrideDocumentTitle meta?
  YES → Skip (ProjectRouteShell / SettingRouteShell manages title)
  NO  → Set document title from route.meta.title(route)
        Fallback: default "Bytebase" title
```
