# ISS-AI-001 — Hybrid Vue + React Framework Gây Nhầm Lẫn Cho AI

> **Category**: Architecture Complexity  
> **Severity**: Critical  
> **Impact**: Code Generation, Refactoring, Bug Fixing  
> **Affected Area**: Toàn bộ frontend — `src/views/`, `src/react/`, `src/components/`, `src/react/components/`

---

## 1. Mô Tả Vấn Đề

Codebase sử dụng kiến trúc **hybrid dual-framework**: Vue 3 làm shell chính (router, layouts, legacy components) và React 19 cho các trang mới. Sự tồn tại song song của hai framework tạo ra nhiều điểm mù cho AI:

### 1.1 Nhầm Lẫn Syntax và Pattern

- AI phải phân biệt giữa **Vue SFC** (`.vue` — `<template>`, `<script setup>`, `<style>`) và **React TSX** (`.tsx`).
- Cùng một chức năng (ví dụ: `ExprEditor`) tồn tại ở cả hai framework: `src/components/ExprEditor/` (Vue) và `src/react/components/ExprEditor.tsx` (React).
- AI dễ generate code Vue syntax trong file React, hoặc ngược lại.

### 1.2 State Sharing Không Tường Minh

- React components import **Pinia stores** (Vue) trực tiếp thông qua `useVueState` hook (sử dụng `useSyncExternalStore` + Vue `watch`).
- Cơ chế này yêu cầu AI hiểu cả Vue reactivity (`ref`, `computed`, `watch`) lẫn React state (`useState`, `useEffect`, `useSyncExternalStore`).
- Phát hiện **68 instances** `useVueState` trải rộng trên các file phức tạp nhất (MembersPage: 16, ConnectionPane: 15, DatabaseObjectExplorer: 12).

### 1.3 Bridge Pattern Phi Tiêu Chuẩn

- `ReactPageMount.vue` → `mount.ts` → `import.meta.glob()` → React component.
- Pattern resolution qua 8 directories với fallback order (settings → project → plugins → workspace → auth → components).
- AI không có training data phổ biến về pattern mount React trong Vue shell.

## 2. Ví Dụ Cụ Thể

```
Vue Shell Owner:
  Vue Router → DashboardLayout.vue → BodyLayout.vue (teleport targets)
    → ReactPageMount.vue → mountReactPage(container, "MembersPage")
      → mount.ts → import("./pages/settings/MembersPage.tsx")
        → MembersPage.tsx uses useVueState(() => useAuthStore().currentUser)
```

AI cần trace qua **4 layers** (Vue Router → Vue Layout → Bridge → React Page) để hiểu một page đơn giản.

## 3. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn AI |
|---|---|
| Generate new page | AI phải biết page mới viết React (not Vue), register trong Vue Router, và name phải match export name |
| Modify shared state | AI phải hiểu Pinia store (Vue), `useVueState` bridge (React), và khi nào dùng Zustand vs Pinia |
| Debug rendering issue | AI cần trace qua Vue teleport → React portal → DOM mount lifecycle |
| Add i18n translation | AI phải biết dùng `t()` cho Vue, `useTranslation()` cho React, và sync qua `CustomEvent` |

## 4. Khuyến Nghị Giảm Thiểu

1. **Enforce single-framework context**: Cung cấp cho AI context rõ ràng file nào là Vue, file nào là React (prefix hoặc directory convention).
2. **Document bridge contract**: Tạo `.ai-context` hoặc JSDoc annotation giải thích `useVueState` và `mountReactPage` flows.
3. **Limit `useVueState` scope**: Dần migrate high-usage Pinia stores sang Zustand để giảm cross-framework dependency.
4. **AI-specific AGENTS.md enhancement**: Thêm explicit routing decision tree cho AI (new code → React, existing Vue → preserve, shared state → Pinia via `useVueState`).
