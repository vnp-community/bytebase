/**
 * Route Registry Whitelist (TASK-W-022)
 *
 * Replaces hardcoded `startsWith` whitelist with a Set-based registry.
 * Routes self-register via `meta: { registered: true }`.
 */

import type { Router, RouteLocationNormalized } from "vue-router";
import type { GuardFn, GuardResult } from "./index";
import { guardContinue, guardRedirect } from "./index";
import { WORKSPACE_ROUTE_404 } from "../dashboard/workspaceRoutes";

const registeredRoutes = new Set<string>();

/**
 * Register a route name as allowed.
 */
export function registerAllowedRoute(name: string): void {
  registeredRoutes.add(name);
}

/**
 * Check if a route name is in the registry.
 */
export function isRouteAllowed(name: string): boolean {
  return registeredRoutes.has(name);
}

/**
 * Auto-register all routes that have `meta.registered === true`
 * from the router's route list.
 */
export function autoRegisterRoutes(router: Router): void {
  for (const route of router.getRoutes()) {
    if (route.meta?.registered === true && route.name) {
      registeredRoutes.add(route.name as string);
    }
  }
}

/**
 * Guard that checks the route registry.
 * - Routes with `meta.registered === true` are allowed.
 * - Named routes in the Set are allowed.
 * - Unknown routes are redirected to 404 with a diagnostic log.
 */
export const routeRegistryGuard: GuardFn = (
  to: RouteLocationNormalized
): GuardResult => {
  const routeName = to.name?.toString();
  if (!routeName) return guardContinue();

  // Routes that self-register via meta
  if (to.meta?.registered === true) return guardContinue();

  // Routes previously registered
  if (isRouteAllowed(routeName)) return guardContinue();

  // Unknown route — log and redirect to 404
  console.warn("[RouteRegistry] Unregistered route:", {
    name: routeName,
    path: to.path,
    fullPath: to.fullPath,
  });

  return guardRedirect({
    name: WORKSPACE_ROUTE_404,
    replace: false,
  });
};

/**
 * Get the count of registered routes (for diagnostics).
 */
export function getRegisteredRouteCount(): number {
  return registeredRoutes.size;
}
