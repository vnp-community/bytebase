# BUG-WEAK-003: Authentication & Security Vulnerabilities

> **Severity**: HIGH  
> **Category**: Security Bug  
> **Affected Files**: `src/store/modules/v1/auth.ts`, `src/connect/middlewares/authInterceptorMiddleware.ts`, `src/connect/refreshToken.ts`, `src/router/index.ts`  
> **Status**: OPEN  
> **Created**: 2026-05-13

---

## 1. Mô tả

Nhiều lỗ hổng bảo mật trong luồng authentication, token refresh, và session management có thể dẫn đến unauthorized access hoặc session hijacking.

---

## 2. Chi tiết lỗi

### 2.1 Silent Error Swallowing trong Auth Store

**File**: `src/store/modules/v1/auth.ts` (L219-227)

```typescript
const fetchCurrentUser = async () => {
  try {
    const user = await userStore.fetchCurrentUser();
    currentUserName.value = user.name;
    return user;
  } catch {
    // do nothing.  <-- ALL errors silently swallowed
  }
};
```

**Vấn đề**:
- **Mọi lỗi** (network failure, server error, malformed response) đều bị nuốt im lặng.
- Nếu server trả lỗi 500, user vẫn ở trạng thái "logged out" mà không có thông báo.
- Critical: nếu `fetchCurrentUser` fail do token expired nhưng không phải 401 (ví dụ: 503 Server Error), flow không trigger token refresh.

### 2.2 Logout Error Suppression

**File**: `src/store/modules/v1/auth.ts` (L198-216)

```typescript
const logout = async () => {
  try {
    await authServiceClientConnect.logout({});
  } catch {
    // nothing  <-- Server-side session not properly terminated
  } finally {
    // ...proceeds to cleanup
  }
};
```

**Vấn đề**:
- Nếu logout API call fail, server-side session **không bị invalidate**.
- Client xóa local state nhưng session token vẫn valid trên server → session có thể bị replay.

### 2.3 Open Redirect qua `redirectParam` (Partial Fix)

**File**: `src/router/index.ts` (L151-163)

```typescript
const relayState = to.query["relay_state"] as string | undefined;
const redirectParam = to.query["redirect"] as string | undefined;

let redirect = "/";
if (relayState && typeof relayState === "string") {
  if (relayState.startsWith("/") && !relayState.startsWith("//")) {
    redirect = relayState;
  }
} else if (redirectParam) {
  redirect = redirectParam;  // <-- NO validation for redirect param!
}
```

**Vấn đề**:
- `relay_state` được validate đúng (chỉ cho relative URLs, block `//`).
- Nhưng `redirectParam` **không được validate** → attacker có thể inject `?redirect=https://evil.com` → open redirect vulnerability.
- `redirect` query param được dùng rộng rãi trong login/logout flows.

### 2.4 Token Refresh Race Window

**File**: `src/connect/refreshToken.ts` (L41-56)

```typescript
async function tryAcquireAndRefresh(): Promise<boolean> {
  return navigator.locks.request(LOCK_NAME, { ifAvailable: true }, async (lock) => {
    if (!lock) { return false; }
    await authServiceClientConnect.refresh({});
    const channel = new BroadcastChannel(CHANNEL_NAME);
    channel.postMessage("complete");
    channel.close();
    return true;
  });
}
```

**Vấn đề**:
- **Chỉ broadcast trên success** — nếu refresh fail, các tab khác phải đợi timeout 10s rồi retry.
- Nếu refresh fail liên tục, **tất cả tabs đều bị stuck 10s mỗi lần** = poor UX.
- Không có mechanism để broadcast failure → waiting tabs không biết nên hiển thị login prompt ngay.

### 2.5 Web Locks API Fallback Missing

**File**: `src/connect/refreshToken.ts`

**Vấn đề**:
- `navigator.locks` có **limited browser support** (không hỗ trợ trên một số mobile browsers cũ, Firefox < 96).
- Không có fallback mechanism — nếu `navigator.locks` undefined → **uncaught TypeError** → toàn bộ auth flow crash.

### 2.6 OAuth Event Listener Leak

**File**: `src/App.vue` (L154-160)

```typescript
// Outside any lifecycle hook - added globally, never removed
window.addEventListener("bb.oauth.unknown", () => {
  notificationStore.pushNotification({ ... });
});
```

**Vấn đề**:
- Event listener thêm **ngoài lifecycle hook** → không bao giờ bị remove.
- Mỗi khi `App.vue` remount (unlikely nhưng possible trong testing hoặc hot reload) → duplicate listeners.

---

## 3. Tác động

| Impact | Mô tả |
|--------|--------|
| **Open Redirect** | Attacker inject redirect URL qua login flow → phishing attack vector |
| **Session Persistence** | Server-side session không bị invalidate khi logout fail → replay attack |
| **UX Degradation** | Token refresh failure → 10s freeze trên tất cả tabs |
| **Browser Compat** | Web Locks missing → auth flow crash trên older browsers |
| **Silent Auth Failure** | fetchCurrentUser swallows all errors → user không biết auth state bị corrupted |

---

## 4. Đề xuất Fix

1. **Validate redirectParam** — áp dụng cùng logic validate như `relay_state`
2. **Broadcast refresh failure** — thêm failure message để waiting tabs hiển thị prompt ngay
3. **Add Web Locks polyfill/fallback** — dùng `navigator.locks?.request || fallbackMutex`
4. **Log auth errors** — ít nhất console.error thay vì swallow hoàn toàn
5. **Retry logout** — thêm retry logic cho logout API call trước khi cleanup local state
6. **Move OAuth listener** — vào `onMounted`/`onUnmounted` lifecycle
