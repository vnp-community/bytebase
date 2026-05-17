# TASK-W-021: Guard Pipeline Architecture

> **Source**: SOL-WEAK-008 §2.1 | **Priority**: P3 | **Effort**: 3h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/router/guards/index.ts`

## What
Create `GuardResult` type, `GuardFn` type, and `executeGuardPipeline()` function. Extract existing 9+ branches from `router.beforeEach` into individual composable guard functions.

## Implementation — see SOL-WEAK-008 §2.1
- `GuardResult`: `{ action: "next" | "redirect" | "continue" }`
- `GuardFn`: `(to, from) => GuardResult | Promise<GuardResult>`
- `executeGuardPipeline(guards[], to, from, next)`: iterate, first non-continue wins
- Extract guards: `infiniteLoopGuard`, `errorPageBypassGuard`, `oauthCallbackGuard`, `authRedirectGuard`, `loginEnforcementGuard`, `mfaEnforcementGuard`, `passwordResetGuard`

## AC
- [x] `src/router/guards/index.ts` created
- [x] Pipeline executes guards in order
- [x] Fallback → 404 if no guard handles route
- [x] Existing navigation behavior preserved
