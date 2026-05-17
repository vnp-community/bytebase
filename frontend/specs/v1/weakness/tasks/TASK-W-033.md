# TASK-W-033: Encrypted Token Storage

> **Source**: SOL-WEAK-007 §2.3 | **Priority**: P3 | **Effort**: 2h  
> **Status**: DONE | **Deps**: W-031

## Scope
- **EDIT** `src/auth/token-manager.ts`

## What
Replace plaintext `localStorage.setItem` for refresh token with `storageService.save()` using `encrypt: true`.

## Implementation — see SOL-WEAK-007 §2.3
```diff
-localStorage.setItem("bb_refresh_token", token);
+await storageService.save("refresh-token", token, {
+  namespace: "auth", encrypt: true, ttlMs: 7 * 24 * 60 * 60 * 1000
+});

-const token = localStorage.getItem("bb_refresh_token");
+const token = await storageService.load("refresh-token", {
+  namespace: "auth", encrypt: true
+}, null);
```

## AC
- [x] Refresh token stored encrypted (AES-GCM)
- [x] Token auto-expires after 7 days
- [x] Logout clears auth namespace
- [x] XSS cannot read plaintext token from localStorage
