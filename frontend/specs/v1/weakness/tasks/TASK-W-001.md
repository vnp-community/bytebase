# TASK-W-001: Open Redirect Validation

> **Source**: SOL-WEAK-003 §3.1 | **Priority**: P1 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/utils/redirect-validator.ts`
- **EDIT** `src/router/index.ts` (lines ~151-163)

## What
Tạo utility `validateRedirectUrl()` và apply cho cả `relay_state` lẫn `redirectParam` trong router guard.

## Implementation

### File 1: `src/utils/redirect-validator.ts` (NEW)
```typescript
export function validateRedirectUrl(url: string | undefined | null): string {
  if (!url || typeof url !== "string") return "/";
  const trimmed = url.trim();
  if (!trimmed.startsWith("/")) return "/";
  if (trimmed.startsWith("//")) return "/";
  if (/^\/[^/]/.test(trimmed) === false) return "/";
  if (trimmed.includes("\\")) return "/";
  return trimmed;
}
```

### File 2: `src/router/index.ts` — Apply validation
```diff
+import { validateRedirectUrl } from "@/utils/redirect-validator";

 let redirect = "/";
 if (relayState && typeof relayState === "string") {
-  if (relayState.startsWith("/") && !relayState.startsWith("//")) {
-    redirect = relayState;
-  }
+  redirect = validateRedirectUrl(relayState);
 } else if (redirectParam) {
-  redirect = redirectParam;
+  redirect = validateRedirectUrl(redirectParam);
 }
```

## AC
- [ ] `validateRedirectUrl("https://evil.com")` returns `"/"`
- [ ] `validateRedirectUrl("//evil.com")` returns `"/"`
- [ ] `validateRedirectUrl("javascript:alert(1)")` returns `"/"`
- [ ] `validateRedirectUrl("/settings")` returns `"/settings"`
- [ ] `redirectParam` is now validated in router guard
