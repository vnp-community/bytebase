# Change Request: Frontend Migration Acceleration (Vue → React)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-006                                              |
| **Source ID**      | ARCH-LIM-006                                             |
| **Title**          | Frontend Migration Acceleration — Unified React Architecture |
| **Category**       | Architecture (Maintainability + DX)                      |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SQL-01 (SQL Editor), DCM-01 (Change Management UI)      |

---

## 1. Tổng quan

### 1.1 Mô tả
Đẩy nhanh migration từ dual framework (Vue 3 + React 19) sang unified React architecture. Hiện tại 2 runtime, 2 state managers, 2 component libraries, bridge layer `useVueState()` tạo ra bundle bloat và cognitive overhead.

### 1.2 Bối cảnh
- Vue 3.5 (Naive UI, Pinia) + React 19 (Base UI, Zustand) coexist
- `useVueState()` bridge: React subscribes to Vue reactive state
- Double bundle: Vue + React runtimes (~30-50KB gzipped overhead)
- Two testing patterns, two i18n systems, two state stores
- PRD Roadmap signals: "Vue → React migration in progress"

### 1.3 Mục tiêu
- Establish clear migration plan with page-by-page conversion schedule
- Reduce bridge layer dependency to zero
- Bundle size reduced ≥ 30KB (gzipped) after Vue removal
- Single testing + i18n + state management pattern

---

## 2. Yêu cầu chức năng

### FR-001: Migration Priority Matrix
- **Mô tả**: Prioritize page conversion based on complexity and user impact.
- **Priority Map**:

  | Priority | Pages | Justification |
  |----------|-------|---------------|
  | P0 | SQL Editor | Highest usage, already uses Monaco (framework-agnostic) |
  | P1 | Issue/Plan/Rollout Views | Core DCM workflow, high user traffic |
  | P2 | Project/Instance Management | CRUD pages, simpler conversion |
  | P3 | Settings/Admin | Low frequency, complex forms |
  | P4 | Schema Diagram/Editor | Specialized rendering (ELK.js) |

- **Acceptance Criteria**:
  - AC-1: Each priority has estimated conversion effort (person-days)
  - AC-2: Shared components (layout, nav, auth) converted first

### FR-002: State Migration (Pinia → Zustand)
- **Mô tả**: Migrate Pinia stores to Zustand, remove `useVueState()` bridge.
- **Logic**:
  ```typescript
  // BEFORE: React component uses bridge
  const project = useVueState(() => projectStore.currentProject)

  // AFTER: React component uses native Zustand
  const project = useProjectStore(s => s.currentProject)
  ```
- **Acceptance Criteria**:
  - AC-1: All Pinia stores duplicated in Zustand during transition
  - AC-2: New code uses Zustand exclusively
  - AC-3: `useVueState()` removed when all consumers migrated

### FR-003: Component Library Unification
- **Mô tả**: Migrate Naive UI components to Base UI / shadcn equivalents.
- **Acceptance Criteria**:
  - AC-1: Shared component mapping document (NaiveUI → BaseUI)
  - AC-2: Design token alignment between frameworks
  - AC-3: No mixed-framework component trees

### FR-004: Build Optimization
- **Mô tả**: Tree-shake Vue dependencies as pages migrate.
- **Acceptance Criteria**:
  - AC-1: Bundle analyzer tracking per-sprint reduction
  - AC-2: Vue runtime removable after final page migration
  - AC-3: Vite config updated to exclude unused Vue chunks

---

## 3. Yêu cầu kỹ thuật

### 3.1 Frontend Changes

| Component              | Package/Dir                           | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| State stores           | `frontend/src/store/` → Zustand      | Migrate Pinia stores                         |
| Bridge layer           | `frontend/src/react/useVueState`      | Deprecate → remove                           |
| Components             | `frontend/src/components/` → React    | Page-by-page conversion                      |
| i18n                   | `vue-i18n` → `react-i18next` only    | Consolidate to single i18n system            |
| Router                 | Vue Router → React Router             | When all pages converted                      |

### 3.2 Backend/Database Changes
Không có.

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Migrated page has identical functionality                    | Feature parity verified                  |
| TC-002     | Bundle size decreases after each conversion batch            | Measurable reduction                     |
| TC-003     | Bridge layer unused after all pages converted                | `useVueState` has 0 imports              |
| TC-004     | State consistency between Pinia and Zustand during transition | No data divergence                       |
| TC-005     | i18n strings identical in React components                   | Translation complete                     |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Shared components (Layout, Nav, Auth) to React     | Sprint 1-2   |
| Phase 2 | SQL Editor React shell (Monaco is agnostic)        | Sprint 2-3   |
| Phase 3 | Issue/Plan/Rollout pages                           | Sprint 3-5   |
| Phase 4 | Project/Instance management pages                  | Sprint 5-7   |
| Phase 5 | Settings/Admin pages                               | Sprint 7-8   |
| Phase 6 | Remove Vue runtime + bridge + Pinia + vue-i18n     | Sprint 9     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Feature regression during conversion          | HIGH   | Playwright E2E tests before/after each page         |
| Development velocity slowdown                 | MEDIUM | Parallel tracks: new features in React, migration separate |
| Naive UI components without BaseUI equivalent | MEDIUM | Build custom components or use Radix primitives     |
| Long migration timeline                       | LOW    | Accept 9-sprint timeline, prioritize by user impact |
