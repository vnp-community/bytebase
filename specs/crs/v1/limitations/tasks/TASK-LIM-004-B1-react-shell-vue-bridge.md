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

- [ ] React app renders with router
- [ ] AuthGuard redirects unauthenticated users
- [ ] VueBridge mounts Vue app for unmigrated routes
- [ ] URL changes sync between React and Vue routers
- [ ] Vite builds successfully with both Vue and React plugins
