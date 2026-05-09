# Change Request: Frontend Performance for Large-Scale Database Management

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-007                                              |
| **Title**          | Frontend Virtualization & Progressive Loading for 200K DBs |
| **Category**       | Performance / Frontend                                   |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SQL-01, ADM-04, ADM-08                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Frontend (Vue 3 + React 19) hiện render database lists synchronously. Với 200K databases và 100+ tenants, UI trở nên unresponsive khi listing, searching, hoặc navigating database trees. Schema diagram render timeout với databases có >500 tables.

### 1.2 Bối cảnh
- Database list page loads tất cả databases trước khi render (blocking)
- Tree view (project → instance → database) không virtualized
- Search debounce nhưng vẫn query full dataset mỗi keystroke
- Schema diagram render toàn bộ tables — crash khi >500 tables
- State management (Pinia/Zustand) stores toàn bộ database list in memory

### 1.3 Mục tiêu
- Initial page load ≤ 2s cho database list (any scale)
- Smooth scrolling qua 200K database items
- Search results ≤ 300ms for prefix queries
- Schema diagram render ≤ 3s cho databases có 1000 tables

---

## 2. Yêu cầu chức năng

### FR-001: Virtual Scrolling cho Database Lists
- **Mô tả**: Replace full list render với virtual scrolling (render only visible rows)
- **Implementation**: React `@tanstack/react-virtual` / Vue `vue-virtual-scroller`
- **AC**:
  - AC-1: Render 200K items with ≤ 50 DOM nodes at any time
  - AC-2: Scroll performance ≥ 60fps
  - AC-3: Keyboard navigation (up/down/enter) works seamlessly

### FR-002: Progressive Data Loading
- **Mô tả**: Load database data in chunks with infinite scroll
- **Logic**: Initial load 50 items → scroll to bottom → load next 50
- **AC**:
  - AC-1: Initial render ≤ 500ms
  - AC-2: Next page loads ≤ 200ms (prefetched)
  - AC-3: Total count displayed immediately via COUNT query

### FR-003: Server-Side Search & Filtering
- **Mô tả**: Move search/filter to backend API thay vì client-side filtering
- **AC**:
  - AC-1: Search results appear ≤ 300ms after typing stops (debounce 200ms)
  - AC-2: Backend search uses database indexes
  - AC-3: Search highlights matching text

### FR-004: Lazy Schema Diagram Rendering
- **Mô tả**: Render schema diagram progressively — visible tables first, rest on scroll
- **AC**:
  - AC-1: Initial render ≤ 3s cho 1000 tables
  - AC-2: Only visible tables fully rendered (relationships on-demand)
  - AC-3: Zoom-to-fit calculates layout without full render

### FR-005: State Management Optimization
- **Mô tả**: Replace full dataset store với paginated/windowed state
- **Logic**: Store only current page + 1 prefetch page in Pinia/Zustand
- **AC**:
  - AC-1: Memory usage ≤ 50MB regardless of total database count
  - AC-2: Page navigation ≤ 100ms (prefetched)
  - AC-3: Browser tab does not crash at 200K databases

---

## 3. Yêu cầu kỹ thuật

### 3.1 Frontend Changes

| Component          | File/Package                          | Thay đổi                              |
|--------------------|---------------------------------------|----------------------------------------|
| Database List      | `frontend/src/views/DatabaseList`     | Virtual scrolling + progressive load   |
| Database Tree      | `frontend/src/components/DatabaseTree`| Virtualized tree view                  |
| Schema Diagram     | `frontend/src/views/SchemaDiagram`    | Lazy rendering + viewport culling      |
| Search Component   | `frontend/src/components/SearchBar`   | Server-side search API integration     |
| Store — Database   | `frontend/src/stores/database.ts`     | Paginated state management             |
| API Client         | `frontend/src/api/database.ts`        | Add view param, streaming support      |

### 3.2 Dependencies

| Package                    | Version | Purpose                           |
|----------------------------|---------|-----------------------------------|
| `@tanstack/react-virtual`  | ^3.x    | Virtual scrolling (React)         |
| `vue-virtual-scroller`     | ^2.x    | Virtual scrolling (Vue)           |
| `react-window`             | ^1.x    | Alternative virtualizer           |

---

## 4. Performance Targets

| Metric                        | Current       | Target (200K+ DBs) |
|-------------------------------|---------------|---------------------|
| Database list initial load    | ~5s (10K)     | ≤ 2s (200K)        |
| Scroll FPS                    | ~15fps (10K)  | ≥ 60fps (200K)     |
| Search result latency         | ~1s client    | ≤ 300ms server     |
| Schema diagram render         | crash (500+)  | ≤ 3s (1000 tables) |
| Tab memory usage              | ~300MB (10K)  | ≤ 50MB (200K)      |

---

## 5. Test Cases

| Test ID | Mô tả                                         | Expected Result                |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | Virtual scroll 200K database items             | 60fps, ≤ 50 DOM nodes         |
| TC-002  | Progressive load: scroll to bottom             | Next page loads ≤ 200ms       |
| TC-003  | Search "bank-" across 200K databases           | Results ≤ 300ms               |
| TC-004  | Schema diagram: 1000 tables                    | Render ≤ 3s                   |
| TC-005  | Tab memory after browsing 200K databases       | ≤ 50MB                        |
| TC-006  | Keyboard navigation in virtual list            | Correct selection, smooth      |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline   |
|---------|--------------------------------------|------------|
| Phase 1 | Virtual scrolling + progressive load | Sprint 1   |
| Phase 2 | Server-side search                   | Sprint 2   |
| Phase 3 | Schema diagram lazy render           | Sprint 2   |
| Phase 4 | State management optimization        | Sprint 3   |
| Phase 5 | E2E performance testing              | Sprint 3   |

---

## 7. Risks & Mitigations

| Risk                           | Impact | Mitigation                              |
|--------------------------------|--------|-----------------------------------------|
| Virtual scroll accessibility   | MEDIUM | ARIA attributes + keyboard support      |
| Dual framework (Vue+React)     | HIGH   | Implement for React first, Vue follows  |
| Server search latency spikes   | LOW    | Query timeout + local cache fallback    |
| Schema diagram layout perf     | MEDIUM | Web Worker for layout computation       |
