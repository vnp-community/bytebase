# TASK-W-009: Page Cache Cleanup on Logout

> **Source**: SOL-WEAK-001 §3.5 | **Priority**: P1 | **Effort**: 1h  
> **Status**: DONE | **Deps**: W-007

## Scope
- **EDIT** `src/react/mount.ts` — export `clearPageCache()`
- **EDIT** `src/store/modules/v1/auth.ts` — call `clearPageCache()` in `logout()`

## Implementation

### mount.ts
```diff
 const cachedPages = new Map<string, ReactComponent>();
+
+export function clearPageCache(): void {
+  cachedPages.clear();
+}
```

### auth.ts — in `logout()` finally block
```diff
+import { clearPageCache } from "@/react/mount";
 // ... after store resets
+clearPageCache();
```

## AC
- [ ] `clearPageCache` exported from `mount.ts`
- [ ] Called during `logout()` in auth store
- [ ] After logout + re-login, page components are freshly loaded
