/**
 * Guard Pipeline Architecture (TASK-W-021)
 *
 * Provides a composable guard pipeline for vue-router `beforeEach`.
 * Each guard returns a `GuardResult` indicating whether to proceed,
 * redirect, or continue to the next guard in the pipeline.
 */

import type {
  RouteLocationNormalized,
  NavigationGuardNext,
} from "vue-router";

// ─── Types ───────────────────────────────────────────────────

export type GuardAction = "next" | "redirect" | "continue";

export interface GuardResult {
  action: GuardAction;
  /** Redirect target — only used when action is "redirect". */
  to?: Parameters<NavigationGuardNext>[0];
}

export type GuardFn = (
  to: RouteLocationNormalized,
  from: RouteLocationNormalized
) => GuardResult | Promise<GuardResult>;

// ─── Guard Shortcuts ─────────────────────────────────────────

/** Allow the navigation to proceed — terminates the pipeline. */
export const guardNext = (): GuardResult => ({ action: "next" });

/** Redirect the navigation — terminates the pipeline. */
export const guardRedirect = (
  target: Parameters<NavigationGuardNext>[0]
): GuardResult => ({
  action: "redirect",
  to: target,
});

/** Continue to the next guard in the pipeline — does NOT terminate. */
export const guardContinue = (): GuardResult => ({ action: "continue" });

// ─── Pipeline Executor ───────────────────────────────────────

/**
 * Execute a pipeline of guard functions in order.
 * - First guard returning "next" → calls `next()` and stops.
 * - First guard returning "redirect" → calls `next(to)` and stops.
 * - "continue" → proceed to the next guard.
 * - If all guards return "continue", the pipeline falls through to the fallback.
 */
export async function executeGuardPipeline(
  guards: GuardFn[],
  to: RouteLocationNormalized,
  from: RouteLocationNormalized,
  next: NavigationGuardNext,
  fallback?: GuardResult
): Promise<void> {
  for (const guard of guards) {
    try {
      const result = await guard(to, from);

      switch (result.action) {
        case "next":
          next();
          return;
        case "redirect":
          next(result.to);
          return;
        case "continue":
          break;
      }
    } catch (err) {
      console.error("[GuardPipeline] Guard threw an error:", err);
      // On error, allow navigation to prevent blocking the user
      next();
      return;
    }
  }

  // All guards returned "continue" — apply fallback
  if (fallback) {
    if (fallback.action === "redirect") {
      next(fallback.to);
    } else {
      next();
    }
  } else {
    next();
  }
}
