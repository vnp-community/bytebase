# BUG-WEAK-005: Error Handling Anti-Patterns

> **Severity**: MEDIUM  
> **Category**: Error Handling Bug  
> **Status**: OPEN | **Created**: 2026-05-13

## 1. Mô tả

Hệ thống có nhiều anti-patterns trong error handling — silent error swallowing, inconsistent error boundaries, và missing error propagation.

## 2. Chi tiết lỗi

### 2.1 ConnectError Blanket Suppression
**File**: `src/App.vue` (L130-148)

```typescript
onErrorCaptured((error: unknown) => {
  if (error instanceof ConnectError && Object.values(Code).includes(error.code)) {
    return; // ALL ConnectErrors with valid codes are silently suppressed
  }
  // ...
  return true; // returns true → error NOT propagated to parent
});
```

- **Mọi ConnectError có valid gRPC code** đều bị suppress — kể cả `Code.Internal` (500), `Code.DataLoss`, `Code.Unavailable`.
- `return true` ngăn error propagation → nếu interceptor bỏ lỡ error, nó biến mất hoàn toàn.

### 2.2 Error Notification Middleware Default Suppression
**File**: `src/connect/middlewares/errorNotificationMiddleware.ts` (L37-42)

```typescript
if ((ignoredCodes.length === 0
  ? [Code.NotFound, Code.Unauthenticated]
  : ignoredCodes
).includes(error.code)) {
  // ignored
}
```

- Mặc định suppress `Code.NotFound` → nếu resource bị xóa, user không thấy thông báo, chỉ thấy empty/stale UI.

### 2.3 Empty Catch Blocks Across Codebase
Nhiều file có empty catch blocks:
- `src/store/modules/v1/auth.ts` L202, L225 — `// nothing`, `// do nothing`
- `src/store/modules/sqlEditor/worksheet.ts` L382 — `// Nothing`
- `src/utils/web-storage.ts` L39, L58 — `// nothing`
- `src/plugins/ai/store/conversation.ts` L276 — `// nothing todo`
- `src/views/sql-editor/EditorCommon/ResultView/SingleResultViewV1.vue` L742

### 2.4 Missing React Error Boundary
**File**: `src/react/mount.ts` — React pages mount **without Error Boundary**.
- Nếu React component throw → crash propagates through Vue `onErrorCaptured` → bị suppress.
- No fallback UI cho React page crashes.

## 3. Đề xuất Fix
1. Thay blanket `ConnectError` suppression bằng explicit list các codes cần suppress
2. Thêm React `ErrorBoundary` wrapper trong `buildTree()` function
3. Thay empty catches bằng structured logging (`console.warn`)
4. Default `Code.NotFound` → hiển thị "Resource not found" thay vì silent
