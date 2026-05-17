# TASK-L-008: Redirect URL Validator

> **Source**: SOL-LIM-006 §2.2 | **Priority**: P1 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/utils/redirect-validator.ts`
- **EDIT** `src/router/index.ts` (redirect handling ~L151-163)

## What
Tạo `sanitizeRedirectUrl()` utility và apply cho cả `relay_state` lẫn `redirectParam` trong router guard.

## Implementation

### File 1: `src/utils/redirect-validator.ts` (NEW)
```typescript
export function isValidRedirectUrl(url: string): boolean {
  if (!url || typeof url !== "string") return false;
  if (!url.startsWith("/")) return false;
  if (url.startsWith("//")) return false;
  const decoded = decodeURIComponent(url);
  if (decoded.startsWith("//")) return false;
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) return false;
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(decoded)) return false;
  if (url.includes("\0") || url.includes("%00")) return false;
  return true;
}

export function sanitizeRedirectUrl(url: string | undefined | null): string {
  if (!url || !isValidRedirectUrl(url)) return "/";
  return url;
}
```

### File 2: `src/router/index.ts` — Apply validation
```diff
+import { sanitizeRedirectUrl } from "@/utils/redirect-validator";

 let redirect = "/";
 if (relayState && typeof relayState === "string") {
-  if (relayState.startsWith("/") && !relayState.startsWith("//")) {
-    redirect = relayState;
-  }
+  redirect = sanitizeRedirectUrl(relayState);
 } else if (redirectParam) {
-  redirect = redirectParam;
+  redirect = sanitizeRedirectUrl(redirectParam);
 }
```

## AC
- [ ] `sanitizeRedirectUrl("https://evil.com")` returns `"/"`
- [ ] `sanitizeRedirectUrl("//evil.com")` returns `"/"`
- [ ] `sanitizeRedirectUrl("javascript:alert(1)")` returns `"/"`
- [ ] `sanitizeRedirectUrl("/settings")` returns `"/settings"`
- [ ] `redirectParam` is now validated (was unvalidated)
