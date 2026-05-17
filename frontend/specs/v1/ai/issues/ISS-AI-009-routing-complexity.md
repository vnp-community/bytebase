# ISS-AI-009 — Routing System Phức Tạp Với 100+ Routes và Navigation Guards

> **Category**: Routing Complexity  
> **Severity**: Medium  
> **Impact**: Navigation, Page Creation, Permission Integration  
> **Affected Area**: `src/router/`, `src/react/router/`

---

## 1. Mô Tả Vấn Đề

### 1.1 Route Architecture

- **Vue Router** sở hữu toàn bộ routing (100+ routes).
- React pages được mount qua `ReactPageMount.vue` — không có React Router riêng (trừ standalone mode).
- 30+ project-scoped routes nested 3-4 levels deep.

### 1.2 Navigation Guards Chain

Vue Router `beforeEach` thực hiện **9 checks** tuần tự:
1. Infinite loop prevention
2. Error pages bypass
3. OAuth callback bypass
4. Auth redirect
5. Login enforcement
6. 2FA enforcement
7. Password reset enforcement
8. Route whitelist check
9. Fallback → 404

### 1.3 Named Views + Teleport

```
DashboardLayout sử dụng named views:
  route.components = {
    banner: BannersWrapper,
    body: BodyLayout → { leftSidebar: Sidebar, content: Page, quickstart: Widget }
  }
```

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **Add new route** | AI phải: define route in dashboard/*.ts → set meta.requiredPermissionList → register React page in mount.ts glob |
| **Permission gating** | AI phải biết: route meta → RoutePermissionGuardShell → PermissionStore |
| **Debug redirect loop** | AI cần trace 9 guards sequentially |
| **Page not rendering** | Could be: wrong route name, missing glob pattern, wrong named export |

## 3. Khuyến Nghị

1. **Route registration checklist**: Step-by-step cho "add new page".
2. **Route visualization**: Auto-generate route tree from code.
3. **Guard documentation**: Flowchart cho navigation guard chain.
