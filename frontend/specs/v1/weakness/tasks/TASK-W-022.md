# TASK-W-022: Route Registry Whitelist

> **Source**: SOL-WEAK-008 §2.2 | **Priority**: P3 | **Effort**: 2.5h  
> **Status**: DONE | **Deps**: W-021

## Scope
- **NEW** `src/router/guards/route-registry.ts`
- **EDIT** route definition files to add `meta: { registered: true }`

## What
Replace hardcoded `startsWith` whitelist with `Set`-based registry. Routes self-register via meta marker.

## Implementation — see SOL-WEAK-008 §2.2
- `registerAllowedRoute(name)` / `isRouteAllowed(name)`
- Auto-register from `router.getRoutes()` where `meta.registered === true`
- `routeRegistryGuard`: check registry, fallback 404 with `console.warn`

## AC
- [x] No more hardcoded route name strings in whitelist
- [x] Routes register via `meta: { registered: true }`
- [x] Unregistered routes → 404 with diagnostic log
- [x] All existing routes still accessible
