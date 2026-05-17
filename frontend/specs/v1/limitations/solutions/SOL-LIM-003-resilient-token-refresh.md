# SOL-LIM-003 — Resilient Cross-Tab Token Refresh

> **Resolves**: BUG-LIM-003 (Token Refresh Cross-Tab Edge Cases)  
> **Type**: Security Fix + Architectural Change  
> **Priority**: Critical  
> **Effort**: Medium (~1 tuần)  
> **Status**: Proposed

---

## 1. Mục Tiêu

1. Loại bỏ infinite lock starvation qua **lock timeout** mechanism.
2. Sửa BroadcastChannel message loss qua **eager listener** pattern.
3. Loại bỏ XSS attack surface qua **encrypted refresh token storage**.
4. Ngăn mutation duplication qua **idempotency-aware retry** trong auth interceptor.

---

## 2. Giải Pháp Kỹ Thuật

### 2.1 Resilient Lock Pattern (refreshToken.ts)

```typescript
// src/connect/refreshToken.ts — REDESIGNED

const LOCK_NAME = "bb_token_refresh";
const CHANNEL_NAME = "bb_token_refresh";
const LOCK_TIMEOUT_MS = 30_000;    // Max time to hold lock
const BROADCAST_WAIT_MS = 10_000;  // Wait for broadcast before retry
const MAX_RETRIES = 2;

let localPromise: Promise<void> | null = null;

export async function refreshTokens(): Promise<void> {
  if (localPromise) return localPromise;
  localPromise = doRefreshWithRetry().finally(() => { localPromise = null; });
  return localPromise;
}

async function doRefreshWithRetry(attempt = 0): Promise<void> {
  // CRITICAL FIX: Create BroadcastChannel BEFORE trying lock
  // This ensures we catch "complete" messages even if leader finishes
  // between our lock check and channel creation.
  const channel = new BroadcastChannel(CHANNEL_NAME);
  
  try {
    const acquired = await tryAcquireWithTimeout();
    if (acquired) {
      channel.close();
      return; // We were the leader, refresh completed
    }
    
    // Wait for leader's broadcast
    const received = await waitForMessage(channel, BROADCAST_WAIT_MS);
    if (received) return;
    
    // Timeout → retry
    if (attempt < MAX_RETRIES) {
      return doRefreshWithRetry(attempt + 1);
    }
    throw new Error("Token refresh failed after retries");
  } finally {
    try { channel.close(); } catch {}
  }
}

async function tryAcquireWithTimeout(): Promise<boolean> {
  return Promise.race([
    navigator.locks.request(LOCK_NAME, { ifAvailable: true }, async (lock) => {
      if (!lock) return false;
      await authServiceClientConnect.refresh({});
      // Broadcast success
      const bc = new BroadcastChannel(CHANNEL_NAME);
      bc.postMessage("complete");
      bc.close();
      return true;
    }),
    // CRITICAL FIX: Timeout prevents infinite lock hold
    new Promise<boolean>((_, reject) =>
      setTimeout(() => reject(new Error("Lock timeout")), LOCK_TIMEOUT_MS)
    ),
  ]);
}

function waitForMessage(channel: BroadcastChannel, timeoutMs: number): Promise<boolean> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(false), timeoutMs);
    channel.onmessage = () => {
      clearTimeout(timer);
      resolve(true);
    };
  });
}
```

**Key changes:**
- BroadcastChannel created BEFORE lock attempt → no message loss window.
- `Promise.race` with timeout → lock holder cannot block forever.
- Retry with max attempts → eventual failure is explicit, not silent.

### 2.2 Idempotency-Aware Auth Interceptor

```typescript
// src/connect/middlewares/authInterceptorMiddleware.ts — UPDATED

// Methods considered safe to retry (no side effects)
const SAFE_TO_RETRY_PREFIXES = ["Get", "List", "Search", "Batch", "Check"];

function isSafeToRetry(methodName: string): boolean {
  return SAFE_TO_RETRY_PREFIXES.some((prefix) => methodName.startsWith(prefix));
}

export const authInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (error) {
    if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
      // Skip Login/Signup/Refresh
      if (["Login", "Signup", "Refresh"].includes(req.method.name)) throw error;

      try {
        await refreshTokens();
      } catch (e) {
        handleUnauthenticatedFailure({ silent, isLoggedIn });
        throw error;
      }

      // CRITICAL FIX: Only retry idempotent (read) requests
      if (isSafeToRetry(req.method.name)) {
        try {
          return await next(req);
        } catch (retryError) {
          if (retryError instanceof ConnectError && retryError.code === Code.Unauthenticated) {
            handleUnauthenticatedFailure({ silent, isLoggedIn });
          }
          throw retryError;
        }
      }
      
      // For mutations: refresh succeeded, but don't retry the mutation
      // Let the caller handle retry if needed
      throw error;
    }
    // ... rest of error handling
    throw error;
  }
};
```

### 2.3 Encrypted Refresh Token (Token Mode)

```typescript
// src/auth/token-manager.ts — UPDATED storage

const REFRESH_TOKEN_KEY = "bb_rt_enc";
const ENCRYPTION_KEY_NAME = "bb_rt_key";

async function getOrCreateEncryptionKey(): Promise<CryptoKey> {
  // Use IndexedDB for CryptoKey storage (not accessible via XSS string extraction)
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

export async function setTokens(access: string, refresh: string): Promise<void> {
  accessToken = access;
  
  // Encrypt refresh token before storing
  const key = await getOrCreateEncryptionKey();
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const encoded = new TextEncoder().encode(refresh);
  const encrypted = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv }, key, encoded
  );
  
  // Store IV + ciphertext (XSS cannot decrypt without CryptoKey)
  const payload = JSON.stringify({
    iv: Array.from(iv),
    ct: Array.from(new Uint8Array(encrypted)),
  });
  localStorage.setItem(REFRESH_TOKEN_KEY, payload);
  scheduleRefresh(access);
}

export async function getRefreshToken(): Promise<string | null> {
  const payload = localStorage.getItem(REFRESH_TOKEN_KEY);
  if (!payload) return null;
  
  const { iv, ct } = JSON.parse(payload);
  const key = await getOrCreateEncryptionKey();
  const decrypted = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: new Uint8Array(iv) },
    key, new Uint8Array(ct)
  );
  return new TextDecoder().decode(decrypted);
}
```

**Security model**: XSS can read the encrypted blob from localStorage but cannot decrypt it — `CryptoKey` with `extractable: false` is stored in IndexedDB and cannot be serialized to a string.

---

## 3. Thay Đổi Architecture Document

### 3.1 Cập nhật `specs/architecture.md` — Section 6.4 Cross-Tab Token Refresh

**Thay thế** nội dung hiện tại bằng:

> ### 6.4 Cross-Tab Token Refresh
>
> `refreshToken.ts` sử dụng **Resilient Lock Pattern**:
>
> 1. Tạo `BroadcastChannel` listener TRƯỚC khi thử acquire lock (eager listener).
> 2. Tab đầu tiên acquire `Web Lock` → call `/v1/auth/refresh`.
> 3. Lock có timeout 30s — nếu tab leader crash, lock tự release.
> 4. Sau khi thành công → broadcast `"complete"` qua BroadcastChannel.
> 5. Các tab khác nhận broadcast → done. Timeout 10s → retry (max 2 lần).
>
> **Idempotency**: Auth interceptor chỉ retry **read requests** (`Get*`, `List*`, `Search*`) sau token refresh. Mutation requests KHÔNG được auto-retry để ngăn duplicate side effects.

### 3.2 Cập nhật `specs/architecture.md` — Section 9.2 Dual Auth Modes

Thêm row mới cho Token mode:

> | **Token (Standalone)** | `Authorization: Bearer` header | Access token in memory, refresh token **encrypted** in localStorage (AES-GCM via Web Crypto API, key in IndexedDB) |

### 3.3 Cập nhật `specs/technical-design-document.md` — Section 3.2

Thêm subsection "Retry Safety":

> **Retry Safety**: Auth interceptor phân loại methods thành safe-to-retry (prefixes: `Get`, `List`, `Search`, `Batch`, `Check`) và mutations (tất cả còn lại). Chỉ safe methods được auto-retry sau token refresh. Mutations throw error cho caller xử lý, ngăn chặn duplicate database changes.

---

## 4. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Lock starvation (tab crash) | Infinite block | 30s timeout + retry |
| BroadcastChannel message loss | Possible | Eliminated (eager listener) |
| Refresh token XSS extraction | Plaintext in localStorage | Encrypted (AES-GCM) |
| Mutation retry duplication | Possible | Impossible (read-only retry) |
| Max retry attempts | Unlimited | 2 (then explicit failure) |
