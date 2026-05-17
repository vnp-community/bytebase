# TASK-L-003: Encrypted Refresh Token Storage

> **Source**: SOL-LIM-003 §2.3 | **Priority**: P1 | **Effort**: 3h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/auth/token-manager.ts` (setTokens, getRefreshToken functions)

## What
Encrypt refresh token trước khi lưu vào localStorage bằng AES-GCM (Web Crypto API). CryptoKey lưu trong IndexedDB với `extractable: false` → XSS không thể extract.

## Implementation

### Thêm encryption helpers
```typescript
const REFRESH_TOKEN_KEY = "bb_rt_enc";
const ENCRYPTION_KEY_NAME = "bb_rt_key";

async function getOrCreateEncryptionKey(): Promise<CryptoKey> {
  const db = await openDB("bb_auth", 1);
  let key = await db.get("keys", ENCRYPTION_KEY_NAME);
  if (!key) {
    key = await crypto.subtle.generateKey(
      { name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"]
    );
    await db.put("keys", key, ENCRYPTION_KEY_NAME);
  }
  return key;
}
```

### Update setTokens()
```diff
 export async function setTokens(access: string, refresh: string): Promise<void> {
   accessToken = access;
-  localStorage.setItem(REFRESH_TOKEN_KEY, refresh);
+  const key = await getOrCreateEncryptionKey();
+  const iv = crypto.getRandomValues(new Uint8Array(12));
+  const encoded = new TextEncoder().encode(refresh);
+  const encrypted = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, encoded);
+  const payload = JSON.stringify({
+    iv: Array.from(iv),
+    ct: Array.from(new Uint8Array(encrypted)),
+  });
+  localStorage.setItem(REFRESH_TOKEN_KEY, payload);
   scheduleRefresh(access);
 }
```

### Update getRefreshToken()
```diff
 export async function getRefreshToken(): Promise<string | null> {
   const payload = localStorage.getItem(REFRESH_TOKEN_KEY);
   if (!payload) return null;
-  return payload;
+  const { iv, ct } = JSON.parse(payload);
+  const key = await getOrCreateEncryptionKey();
+  const decrypted = await crypto.subtle.decrypt(
+    { name: "AES-GCM", iv: new Uint8Array(iv) },
+    key, new Uint8Array(ct)
+  );
+  return new TextDecoder().decode(decrypted);
 }
```

## AC
- [ ] Refresh token in localStorage is encrypted (not plaintext)
- [ ] CryptoKey stored in IndexedDB with `extractable: false`
- [ ] XSS script reading localStorage gets cipher blob, not token
- [ ] Existing sessions: graceful migration (catch JSON parse error → clear + re-login)
