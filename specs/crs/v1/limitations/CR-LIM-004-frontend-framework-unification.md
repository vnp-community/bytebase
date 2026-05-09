# Change Request: Frontend Framework Unification — Complete React Migration

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-LIM-004                                               |
| **Limitation ID**  | LIM-004                                                  |
| **Title**          | Frontend Framework Unification — Complete React Migration|
| **Category**       | Frontend / Developer Experience                          |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Hoàn tất migration frontend từ **Vue 3 + Pinia** sang **React 19 + Zustand**, loại bỏ dual-framework complexity, thống nhất UI library, state management, và i18n system.

### 1.2 Bối cảnh
Frontend hiện tại chạy đồng thời Vue 3 và React 19 qua bridge layer (`useVueState()`). Dual framework gây ra: bundle size tăng 30-40%, developer onboarding khó khăn, bridge bugs khó debug, và inconsistent UI. Migration đang "in progress" nhưng thiếu timeline rõ ràng.

### 1.3 Mục tiêu
- Hoàn tất 100% migration Vue → React theo route-by-route strategy
- Loại bỏ Vue runtime, Pinia, Naive UI, vue-i18n khỏi bundle
- Giảm bundle size ≥ 30%
- Thống nhất component library sang Base UI + Tailwind CSS v4
- Single state management system (Zustand)
- Single i18n system (react-i18next)

---

## 2. Yêu cầu chức năng

### FR-001: Route-by-Route Migration Plan
- **Mô tả**: Phân loại tất cả Vue routes và tạo migration backlog theo priority.
- **Priority Tiers**:
  | Tier    | Routes                                          | Rationale                        |
  |---------|--------------------------------------------------|----------------------------------|
  | Tier 1  | SQL Editor, Schema Editor, Schema Diagram        | Highest user interaction         |
  | Tier 2  | Issue/Plan/Rollout Management                    | Core workflow                    |
  | Tier 3  | Instance/Database/Project Management             | CRUD pages, straightforward      |
  | Tier 4  | Settings, Administration, User Management        | Lower complexity                 |
- **Acceptance Criteria**:
  - AC-1: Complete inventory of Vue routes với size estimation
  - AC-2: Migration backlog in project management tool
  - AC-3: Each route has clear owner và sprint assignment

### FR-002: Bridge Layer Elimination
- **Mô tả**: Loại bỏ `useVueState()` bridge và thay thế bằng native Zustand stores.
- **Migration Pattern**:
  ```
  Vue Pinia Store → Zustand Store (1:1 mapping)
  useVueState() hook → useStore() hook (standard Zustand)
  Vue computed → useMemo / derived selectors
  Vue watchers → useEffect / subscriptions
  ```
- **Acceptance Criteria**:
  - AC-1: Zero `useVueState()` imports remaining
  - AC-2: All Pinia stores have Zustand equivalents
  - AC-3: State synchronization tests pass

### FR-003: UI Component Unification
- **Mô tả**: Migrate tất cả Naive UI components sang Base UI equivalents.
- **Component Mapping**:
  | Naive UI (Vue)    | Base UI (React)      | Priority  |
  |-------------------|----------------------|-----------|
  | NButton           | Button               | P0        |
  | NModal/NDialog    | Dialog               | P0        |
  | NForm/NFormItem   | Form + Field         | P0        |
  | NDataTable        | Table (custom)       | P1        |
  | NSelect           | Select               | P1        |
  | NInput            | Input                | P1        |
  | NTabs             | Tabs                 | P2        |
  | NTree             | Tree (custom)        | P2        |
  | NMenu             | Menu                 | P2        |
- **Acceptance Criteria**:
  - AC-1: Visual regression tests pass (screenshot comparison)
  - AC-2: Accessibility standards maintained (WCAG 2.1 AA)
  - AC-3: All Naive UI imports removed

### FR-004: Router Migration
- **Mô tả**: Migrate từ Vue Router 4 sang React Router v7.
- **Acceptance Criteria**:
  - AC-1: All routes accessible via React Router
  - AC-2: Deep linking works for all pages
  - AC-3: Route guards migrated to React Router loaders/actions
  - AC-4: URL structure unchanged (backward compatible)

### FR-005: i18n Unification
- **Mô tả**: Consolidate vue-i18n và react-i18next sang single react-i18next system.
- **Acceptance Criteria**:
  - AC-1: Single translation file set (JSON namespace structure)
  - AC-2: All vue-i18n `$t()` calls replaced with react-i18next `useTranslation()`
  - AC-3: Missing translation detection in CI pipeline

### FR-006: Build Configuration Cleanup
- **Mô tả**: Simplify Vite + TypeScript configuration to single-framework.
- **Acceptance Criteria**:
  - AC-1: Single `tsconfig.json` (remove `tsconfig.react.json`)
  - AC-2: Vite config without Vue SFC plugin
  - AC-3: Remove `@vitejs/plugin-vue`, `vue-tsc` from dependencies

---

## 3. Yêu cầu kỹ thuật

### 3.1 Migration Architecture

```
Phase 1: Setup React Router + Layout Shell
Phase 2: Migrate routes Tier 1 (SQL Editor, Schema)
Phase 3: Migrate routes Tier 2 (Issue/Plan/Rollout)
Phase 4: Migrate routes Tier 3 (Instance/DB/Project)
Phase 5: Migrate routes Tier 4 (Settings/Admin)
Phase 6: Remove Vue runtime + bridge + cleanup
```

### 3.2 Key Files Affected

| Component           | Current (Vue)                         | Target (React)                        |
|---------------------|---------------------------------------|---------------------------------------|
| App Shell           | `frontend/src/App.vue`               | `frontend/src/App.tsx`               |
| Router              | `frontend/src/router/index.ts`       | `frontend/src/router.tsx`            |
| State Stores        | `frontend/src/store/modules/*.ts`    | `frontend/src/stores/*.ts`           |
| SQL Editor          | `frontend/src/views/sql-editor/`     | `frontend/src/pages/sql-editor/`     |
| Schema Editor       | `frontend/src/views/SchemaEditor*`   | `frontend/src/pages/schema-editor/`  |
| Issue Management    | `frontend/src/views/Issue*`          | `frontend/src/pages/issue/`          |
| i18n                | `frontend/src/i18n/`                 | `frontend/src/i18n/` (restructured)  |

### 3.3 Dependencies to Remove

```json
{
  "remove": [
    "vue", "vue-router", "pinia", "naive-ui",
    "vue-i18n", "@vitejs/plugin-vue", "vue-tsc",
    "@vueuse/core"
  ],
  "keep": [
    "react", "react-dom", "react-router",
    "zustand", "@base-ui-components/react",
    "react-i18next", "i18next"
  ]
}
```

---

## 4. Phụ thuộc

| Dependency          | Mô tả                                                    |
|---------------------|-----------------------------------------------------------|
| Base UI             | React component library (replacement for Naive UI)        |
| React Router v7     | Routing framework                                         |
| Zustand 5.x         | State management                                         |
| react-i18next       | Internationalization                                      |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                        |
|------------|----------------------------------------------------------|----------------------------------------|
| TC-001     | SQL Editor loads and executes query                      | Functional parity with Vue version     |
| TC-002     | Schema Diagram renders relationships                    | Visual parity                          |
| TC-003     | Issue creation workflow                                  | All steps complete successfully        |
| TC-004     | Deep link to specific issue/plan                         | Direct navigation works                |
| TC-005     | i18n: switch language on all pages                       | All text translated                    |
| TC-006     | Bundle size comparison (before vs after)                 | ≥ 30% reduction                       |
| TC-007     | Visual regression test (screenshot diff)                 | < 5% pixel difference                 |
| TC-008     | Accessibility audit (Lighthouse)                         | Score ≥ 90                             |
| TC-009     | Zero Vue imports in final bundle                         | `import ... from 'vue'` count = 0      |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | React Router shell + Tier 4 pages                | Sprint 1-2     |
| Phase 2 | Zustand stores migration                         | Sprint 2-3     |
| Phase 3 | Tier 1 — SQL Editor + Schema (highest risk)      | Sprint 3-6     |
| Phase 4 | Tier 2 — Issue/Plan/Rollout                      | Sprint 6-8     |
| Phase 5 | Tier 3 — Instance/DB/Project CRUD                | Sprint 8-10    |
| Phase 6 | Vue removal + build cleanup + i18n consolidation | Sprint 10-11   |
| Phase 7 | Visual regression + performance testing          | Sprint 11-12   |

---

## 7. Risks & Mitigations

| Risk                                    | Impact | Mitigation                                          |
|-----------------------------------------|--------|------------------------------------------------------|
| Feature regression during migration     | HIGH   | E2E test suite before each phase                     |
| Monaco Editor integration issues        | HIGH   | Isolate Monaco in framework-agnostic wrapper         |
| Timeline exceeds estimate               | MEDIUM | Route-by-route allows partial delivery               |
| State synchronization bugs              | MEDIUM | Comprehensive state comparison tests                 |
| Community contributor disruption        | LOW    | Migration guide + contributor docs                   |
