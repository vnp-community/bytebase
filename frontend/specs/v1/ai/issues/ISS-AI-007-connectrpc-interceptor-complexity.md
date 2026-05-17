# ISS-AI-007 — ConnectRPC + Interceptor Chain Gây Khó Debug Cho AI

> **Category**: API Layer Complexity  
> **Severity**: Medium  
> **Impact**: API Integration, Error Handling, Auth Flows  
> **Affected Area**: `src/connect/`, `src/auth/`

---

## 1. Mô Tả Vấn Đề

### 1.1 ConnectRPC Không Phổ Biến Trong AI Training

ConnectRPC (gRPC-Web) ít phổ biến hơn REST/fetch/axios trong training data. AI thường:
- Generate REST-style `fetch()` calls thay vì ConnectRPC client methods.
- Không biết `createClient(Service, transport)` pattern.
- Nhầm lẫn Protobuf message construction với plain JSON objects.

### 1.2 Interceptor Chain Side Effects

```
Request Flow:
  authInterceptor → injects auth, handles 401 + token refresh
  activeInterceptor → updates lastActiveTs (inactivity detection)
  errorNotificationInterceptor → displays user notification
```

AI thường không biết:
- Errors đã được handle ở interceptor level → double-handling trong component.
- `ConnectError` codes đã filtered ở `App.vue` → component không cần catch.
- Token refresh dùng Web Locks API + BroadcastChannel (cross-tab coordination).

### 1.3 30+ Service Clients

`src/connect/index.ts` exports 30 singleton clients. AI phải chọn đúng client cho domain.

## 2. Giới Hạn

| Scenario | Giới hạn |
|---|---|
| **New API call** | AI generate `fetch('/v1/...')` thay vì ConnectRPC client |
| **Error handling** | AI add try/catch đã handled bởi interceptor |
| **Auth debugging** | AI không hiểu cross-tab token refresh coordination |

## 3. Khuyến Nghị

1. **API call cheat sheet**: Listing `{action} → {client}.{method}({request})` patterns.
2. **Error handling policy doc**: "Don't catch ConnectError in components — interceptors handle it".
3. **Type-safe API examples**: Provide working snippets per domain.
