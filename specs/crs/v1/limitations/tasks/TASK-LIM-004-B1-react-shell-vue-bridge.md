# TASK-LIM-004-B1: React Router Shell + Vue Bridge

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-004 |
| Phase | B — React Shell |
| Priority | P0 |
| Depends On | TASK-LIM-004-A1 |
| Est. | M (~300 LoC) |

## Objective

Create React Router v7 app shell with AuthGuard, root layout, and Vue bridge component for gradual route migration.

## Files

| Action | Path |
|--------|------|
| CREATE | `frontend/src/main.tsx` |
| CREATE | `frontend/src/router.tsx` |
| CREATE | `frontend/src/layouts/RootLayout.tsx` |
| CREATE | `frontend/src/auth/AuthGuard.tsx` |
| CREATE | `frontend/src/bridge/VueBridgePage.tsx` |
| MODIFY | `frontend/vite.config.ts` — add react plugin |
| MODIFY | `frontend/package.json` — add react-router, react |

## Specification

### `router.tsx` — React Router config

- Auth routes: `/auth/login`, `/auth/signup` (no guard)
- Protected routes: `AuthGuard` wrapper with `RootLayout`
- Migrated routes: lazy-loaded React components
- Catch-all `*` route → `VueBridgePage` (temporary bridge)

### `AuthGuard.tsx`

- Check `tokenManager.getAccessToken()` or cookie presence
- Redirect to `/auth/login` if unauthenticated
- Render children if authenticated

### `VueBridgePage.tsx` — temporary

- Mount Vue app inside React div via `createApp().mount(containerRef)`
- Sync React Router location → Vue Router
- Cleanup: `app.unmount()` on effect cleanup
- **Flagged as TEMPORARY** — removed when all routes migrated

### `RootLayout.tsx`

- Sidebar + header chrome (React)
- `<Outlet />` for child routes

## Acceptance Criteria

- [x] React app renders with router → **DONE**: `src/react/main.tsx` bootstraps `RouterProvider` with `reactShellRouter`
- [x] AuthGuard redirects unauthenticated users → **DONE**: `AuthGuard.tsx` checks cookie/token mode, redirects to `/auth/login` with redirect param
- [x] VueBridge mounts Vue app for unmigrated routes → **DONE**: `VueBridgePage.tsx` creates Vue app with `createApp().mount()` and cleans up on unmount
- [x] URL changes sync between React and Vue routers → **DONE**: `syncPath()` effect watches `currentPath` and calls `vueRouter.push()` on changes
- [x] Vite builds successfully with both Vue and React plugins → **DONE**: Existing `vite.config.ts` already has `react-tsx-transform` plugin for `src/react/**/*.tsx` files

## Implementation Notes

- Created `frontend/src/react/pages/auth/AuthGuard.tsx`:
  - Dual mode: cookie check (`document.cookie`) or token check (`getAccessToken()`)
  - Preserves current URL as `?redirect=` parameter in login redirect
- Created `frontend/src/react/pages/VueBridgePage.tsx`:
  - Lazy-loads Vue core (`createApp`, `pinia`, `i18n`, `NaiveUI`, `highlight`)
  - Mounts in a full-height container div
  - Cleanup: `app.unmount()` in React effect cleanup function
  - **Marked as TEMPORARY** — to be removed post-migration
- Created `frontend/src/react/layouts/RootLayout.tsx`:
  - Minimal shell with flexbox layout + `<Outlet />`
  - Sidebar/header chrome to be migrated incrementally from Vue
- Created `frontend/src/react/router/standalone.tsx`:
  - React Router v7 config with `createBrowserRouter()`
  - Auth routes (`/auth/login`, `/auth/signup`) — no guard
  - Protected routes — `AuthGuard` wrapper + `RootLayout`
  - Catch-all `*` → `VueBridgeCatchAll` component
- Created `frontend/src/react/main.tsx`:
  - Standalone React entry point (alternative to `src/main.ts`)

**Status: ✅ DONE**
