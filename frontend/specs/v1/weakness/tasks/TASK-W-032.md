# TASK-W-032: PII-Free Storage Keys

> **Source**: SOL-WEAK-007 §2.2 | **Priority**: P3 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: W-031

## Scope
- **EDIT** `src/utils/storage-service.ts` — add `userScopedKey()` function
- **EDIT** call sites that use `bb.*.{email}` pattern

## What
Replace user email in localStorage keys with hashed identifier. Add `userScopedKey(baseKey, email)` utility.

## Implementation — see SOL-WEAK-007 §2.2
```typescript
export function userScopedKey(baseKey: string, userEmail: string): string {
  let hash = 0;
  for (let i = 0; i < userEmail.length; i++) {
    hash = ((hash << 5) - hash + userEmail.charCodeAt(i)) | 0;
  }
  return `${baseKey}.u${Math.abs(hash).toString(36)}`;
}
// Before: "bb.recent-visit.user@example.com"
// After:  "bb.ui.recent-visit.u7k3m2"
```

## AC
- [x] `userScopedKey` function exported
- [x] No raw email in localStorage keys
- [x] Existing data migrated (old keys cleaned up)
