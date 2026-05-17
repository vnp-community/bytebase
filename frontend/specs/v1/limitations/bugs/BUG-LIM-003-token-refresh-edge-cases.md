# BUG-LIM-003 — Token Refresh Cross-Tab Edge Cases

> **Category**: Authentication / Security  
> **Severity**: Critical  
> **Impact**: Session Loss, Unauthorized Access, UX Disruption  
> **Affected Files**: `src/connect/refreshToken.ts`, `src/connect/middlewares/authInterceptorMiddleware.ts`, `src/auth/token-manager.ts`

---

## 1. Mô Tả Vấn Đề

Hệ thống sử dụng hai cơ chế token refresh song song, tùy thuộc vào auth mode:
- **Cookie mode**: `refreshToken.ts` sử dụng Web Locks API + BroadcastChannel.
- **Token mode**: `token-manager.ts` sử dụng scheduled timeout + localStorage.

Cả hai cơ chế đều có edge cases chưa được xử lý.

### 1.1 Web Locks API — Infinite Wait Khi Tab Leader Crash

```typescript
// refreshToken.ts:41-56
async function tryAcquireAndRefresh(): Promise<boolean> {
  return navigator.locks.request(
    LOCK_NAME,
    { ifAvailable: true },
    async (lock) => {
      if (!lock) return false;  // Lock không available
      await authServiceClientConnect.refresh({});
      // ← Nếu tab này crash/hang ở đây, lock KHÔNG được release
      //   cho đến khi tab bị close bởi browser
      const channel = new BroadcastChannel(CHANNEL_NAME);
      channel.postMessage("complete");
      channel.close();
      return true;
    }
  );
}
```

**Triệu chứng**: Nếu tab đang giữ lock bị treo (heavy computation, network timeout kéo dài), các tab khác sẽ:
1. Fail `tryAcquireAndRefresh()` (lock unavailable)
2. Chờ `waitForBroadcast()` → timeout 10s
3. Retry `tryAcquireAndRefresh()` → vẫn fail (lock chưa release)
4. Token expired → `SessionExpiredSurface` hiển thị trên TẤT CẢ tabs

### 1.2 BroadcastChannel — Message Loss

```typescript
// refreshToken.ts:59-74
function waitForBroadcast(): Promise<boolean> {
  return new Promise((resolve) => {
    const channel = new BroadcastChannel(CHANNEL_NAME);
    const timeout = setTimeout(() => cleanup(false), WAIT_TIMEOUT_MS);
    channel.onmessage = () => {
      clearTimeout(timeout);
      cleanup(true);
    };
  });
}
```

**Vấn đề**: `BroadcastChannel` được tạo SAU khi `tryAcquireAndRefresh()` fail. Nếu tab leader broadcast "complete" TRƯỚC khi tab follower tạo channel → message bị mất → follower timeout → retry không cần thiết.

### 1.3 Token Manager — Refresh Token Trong localStorage (XSS Risk)

```typescript
// token-manager.ts:32-34
export function setTokens(access: string, refresh: string): void {
  accessToken = access;
  localStorage.setItem(REFRESH_TOKEN_KEY, refresh);
  // refresh token trong localStorage → accessible bởi bất kỳ JS code nào trong page
}
```

Mặc dù access token được giữ trong memory, refresh token nằm trong `localStorage`, tạo attack surface cho XSS:
- XSS payload đọc `localStorage.getItem("bb_refresh_token")`
- Gửi refresh token đến attacker server
- Attacker exchange refresh token → nhận access + refresh token mới
- **Persistent session hijacking** (refresh token mới, victim bị revoke)

### 1.4 Auth Interceptor — Retry Không Idempotent

```typescript
// authInterceptorMiddleware.ts:73-87
try {
  await refreshTokens();
} catch (e) {
  // ...
}
try {
  return await next(req);  // ← Retry request SAU refresh
} catch (retryError) {
  // ...
}
```

**Vấn đề**: Interceptor retry request gốc sau khi refresh token. Nếu request ban đầu là **mutation** (create, update, delete) và đã thực thi thành công trên server nhưng response bị 401 do timing issue → retry sẽ **duplicate mutation**.

## 2. Tác Động

| Edge Case | Xác suất | Hậu quả |
|---|---|---|
| Tab leader crash + lock held | Low | Tất cả tabs bị session expired |
| BroadcastChannel message loss | Medium | Unnecessary retry, double API calls |
| XSS + refresh token theft | Low (but critical) | Persistent session hijacking |
| Mutation retry duplication | Low | Duplicate database changes, issues, plans |
| Clock skew + scheduled refresh | Medium | Access token expired trước khi refresh |

## 3. Root Cause

- Web Locks API không có built-in timeout mechanism cho lock holder.
- BroadcastChannel là fire-and-forget, không guarantee delivery.
- Token mode lưu refresh token ở client-side storage thay vì HttpOnly cookie.
- Auth interceptor không phân biệt idempotent vs non-idempotent requests khi retry.

## 4. Khuyến Nghị

1. **Lock timeout**: Sử dụng `Promise.race([lockRequest, timeout(30s)])` để tránh infinite lock.
2. **Eager BroadcastChannel**: Tạo BroadcastChannel listener TRƯỚC khi thử acquire lock.
3. **Migrate refresh token**: Chuyển sang HttpOnly cookie cho token mode (hoặc sử dụng `sessionStorage` + `crypto.subtle` để encrypt).
4. **Idempotency-aware retry**: Chỉ retry cho GET/LIST requests, không retry mutations.
5. **Exponential backoff**: Thay vì fixed 10s timeout, sử dụng exponential backoff cho retry logic.
