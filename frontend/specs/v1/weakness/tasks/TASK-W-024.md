# TASK-W-024: OAuth Consent Guard

> **Source**: SOL-WEAK-008 §2.4 | **Priority**: P3 | **Effort**: 1h  
> **Status**: DONE | **Deps**: W-021

## Scope
- **NEW** `src/router/guards/oauth-consent.ts`

## What
Add auth check for OAuth consent page (currently bypasses all guards).

## Implementation — see SOL-WEAK-008 §2.4
```typescript
const oauthConsentGuard: GuardFn = (to) => {
  if (to.name !== OAUTH2_CONSENT_MODULE) return { action: "continue" };
  if (!useAuthStore().isLoggedIn) {
    return { action: "redirect", to: { name: AUTH_SIGNIN_MODULE, query: { redirect: to.fullPath } } };
  }
  return { action: "next" };
};
```

## AC
- [x] Unauthenticated users redirected to signin from consent page
- [x] Authenticated users can access consent page normally
- [x] Guard integrated into pipeline (W-021)
