# BUG-WEAK-008: Router Guard Complexity & Edge Cases

> **Severity**: MEDIUM  
> **Category**: Navigation Bug  
> **Status**: OPEN | **Created**: 2026-05-13

## 1. Mô tả

Router `beforeEach` guard có 9+ decision branches xử lý auth, 2FA, password reset, whitelist — tạo ra nhiều edge case khó maintain.

## 2. Chi tiết lỗi

### 2.1 Route Whitelist Pattern Matching Brittle
**File**: `src/router/index.ts` (L237-255)

```typescript
const allowedRoutePatterns = [
  ENVIRONMENT_V1_ROUTE_DASHBOARD,
  INSTANCE_ROUTE_DASHBOARD,
  // ...
  "workspace",
  "sql-editor",
];

if (allowedRoutePatterns.some((pattern) =>
  to.name?.toString().startsWith(pattern)
)) { next(); return; }
```

- `startsWith` matching → route named `workspace-admin-hack` cũng match.
- Hardcoded strings mixed với constants → dễ bỏ sót khi thêm route mới.
- Route không trong whitelist → fallback 404 **mà không log tại sao** (chỉ `console.warn`).

### 2.2 Store Reset on Auth Page Visit — Destructive
**File**: `src/router/index.ts` (L170-178)

```typescript
if (isAuthRelatedRoute(to.name as string)) {
  useDatabaseV1Store().reset();
  useProjectV1Store().reset();
  useInstanceV1Store().reset();
  // ...
  next(); return;
}
```

- Navigating tới auth page **clears database, project, instance caches**.
- Nếu user accidentally hits browser back to `/auth/signin` rồi forward → **tất cả cached data bị mất** → re-fetch everything.

### 2.3 OAuth Consent Page Bypasses All Guards
**File**: `src/router/index.ts` (L112-115)

```typescript
if (to.name === OAUTH2_CONSENT_MODULE) {
  next(); return;
}
```

- OAuth consent page bypass **all guards** kể cả auth check.
- Nếu consent page có logic yêu cầu authenticated user → potential issue.

### 2.4 Document Title Race Between Vue & React
**File**: `src/router/index.ts` (L278-291)

```typescript
router.afterEach((to) => {
  if (to.params.projectId || to.meta.overrideDocumentTitle) return;
  nextTick(() => {
    if (to.meta.title) { setDocumentTitle(to.meta.title(to)); }
  });
});
```

- Vue `afterEach` sets title, nhưng React route shells cũng set title (via `overrideDocumentTitle`).
- React mount là async → title bị set bởi Vue trước, rồi overwritten bởi React → title flicker.

## 3. Đề xuất Fix
1. **Registry-based route whitelist** — routes tự đăng ký vào whitelist thay vì hardcode
2. **Lazy store reset** — chỉ reset khi user thực sự logout, không phải visit auth page
3. **OAuth guard** — thêm auth check cho consent page
4. **Unified title management** — single source cho document title
