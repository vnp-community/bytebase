# BUG-LIM-004 — Error Boundary Gaps — React Exceptions Không Được Catch

> **Category**: Error Handling  
> **Severity**: High  
> **Impact**: Blank Page, Uncaught Exceptions, User Data Loss  
> **Affected Files**: `src/App.vue`, `src/react/mount.ts`, `src/react/ReactPageMount.vue`

---

## 1. Mô Tả Vấn Đề

### 1.1 Không Có React Error Boundary

Hệ thống hiện tại KHÔNG có React Error Boundary. Khi một React page/component throw runtime error:

```
Vue Shell (App.vue onErrorCaptured)
  └─ ReactPageMount.vue (container div)
      └─ React Root (createRoot)
          └─ StrictMode → I18nextProvider → PageComponent
              └─ ❌ Runtime Error → React unmounts TOÀN BỘ tree
```

**Hậu quả**: 
- React page bị blank hoàn toàn (React 19 default behavior khi không có Error Boundary).
- Vue shell vẫn render (sidebar, top bar) nhưng content area trống.
- Vue `onErrorCaptured` KHÔNG catch được React errors vì chúng xảy ra ngoài Vue component tree.

### 1.2 ConnectError Silent Swallow

```typescript
// App.vue:130-148
onErrorCaptured((error: unknown) => {
  if (
    error instanceof ConnectError &&
    Object.values(Code).includes(error.code)
  ) {
    return;  // ← SWALLOW tất cả ConnectError có known code
  }
  // ...
});
```

**Vấn đề**: Tất cả gRPC errors với code hợp lệ (bao gồm `INTERNAL`, `DATA_LOSS`, `UNAVAILABLE`) đều bị swallow ở tầng Vue. Điều này có nghĩa:
- Server trả `INTERNAL` error → không notification → user không biết operation failed.
- Chỉ dựa vào `errorNotificationInterceptor` ở tầng ConnectRPC, nhưng interceptor này cũng skip `Unauthenticated` và `NotFound`.

### 1.3 OAuth Event Handler Không Cleanup

```typescript
// App.vue:154-160
window.addEventListener("bb.oauth.unknown", () => {
  notificationStore.pushNotification({
    module: "bytebase",
    style: "CRITICAL",
    title: t("oauth.unknown-event"),
  });
});
// ← KHÔNG có removeEventListener trong onUnmounted
// ← Mỗi lần hot-reload trong dev → thêm 1 listener → duplicate notifications
```

## 2. Tác Động

| Scenario | Xác suất | Hậu quả |
|---|---|---|
| React page runtime error | Medium | Blank content area, user phải refresh |
| gRPC INTERNAL error bị swallow | Medium | Silent data loss, user nghĩ operation thành công |
| React async error (useEffect) | Medium | Unhandled promise rejection, no UI feedback |
| OAuth listener leak (dev mode) | High | Duplicate notifications khi HMR |
| React page load failure | Low | `Unknown React page: X` error, blank page |

## 3. Root Cause

- React application chạy BÊN TRONG Vue DOM nhưng NGOÀI Vue component tree → Vue error handling không áp dụng.
- Thiếu React `ErrorBoundary` component wrapping tại `buildTree()` trong `mount.ts`.
- `onErrorCaptured` filter logic quá rộng — swallow tất cả known gRPC codes thay vì chỉ các codes đã được handle.
- OAuth event listener sử dụng anonymous function → không thể cleanup.

## 4. Khuyến Nghị

1. **Thêm React ErrorBoundary**: Wrap mỗi React page trong ErrorBoundary component:
   ```tsx
   // mount.ts buildTree()
   createElement(ErrorBoundary, { fallback: ErrorFallbackUI },
     createElement(StrictMode, null,
       createElement(I18nextProvider, { i18n },
         createElement(Component, props)
       )
     )
   )
   ```

2. **Refine ConnectError filtering**: Chỉ swallow codes đã có explicit handler:
   ```typescript
   const HANDLED_CODES = [Code.Unauthenticated, Code.PermissionDenied, Code.NotFound];
   if (error instanceof ConnectError && HANDLED_CODES.includes(error.code)) {
     return;
   }
   ```

3. **Cleanup OAuth listener**: Di chuyển handler vào `onMounted/onUnmounted` lifecycle hoặc sử dụng named function.

4. **Global unhandledrejection handler**: Thêm `window.addEventListener("unhandledrejection", ...)` để catch React async errors.
