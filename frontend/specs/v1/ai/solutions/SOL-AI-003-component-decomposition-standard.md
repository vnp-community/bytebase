# SOL-AI-003 — Component Decomposition Standard

> **Resolves**: ISS-AI-003 (God Components Vượt Quá 1000+ LOC)  
> **Type**: Code Architecture + Tooling  
> **Priority**: High  
> **Effort**: Large (ongoing, 2–4 months)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Giảm tất cả components xuống dưới **500 LOC** qua chuẩn hóa decomposition pattern, lint enforcement, và refactoring ưu tiên 18 god components được xác định.

---

## 2. Decomposition Pattern Standard

### 2.1 Container + View + Hooks Split

Áp dụng pattern **Container/View/Hooks** cho tất cả trang:

```
PageName/
├── index.ts                    # Public API (re-exports PageNamePage)
├── PageNamePage.tsx            # Container: chỉ compose hooks + pass props (< 100 LOC)
├── PageNameView.tsx            # View: rendering chỉ (< 300 LOC)
├── components/                 # Sub-components (mỗi file < 200 LOC)
│   ├── PageNameTable.tsx
│   ├── PageNameForm.tsx
│   └── PageNameFilters.tsx
├── hooks/                      # Business logic hooks (mỗi file < 150 LOC)
│   ├── usePageNameData.ts      # Data fetching + cache
│   ├── usePageNameActions.ts   # CRUD operations
│   ├── usePageNameFilters.ts   # Filter/sort/pagination state
│   └── usePageNamePermissions.ts
└── types.ts                    # Local type definitions
```

### 2.2 Ví Dụ Refactoring: `MembersPage.tsx` (1,993 LOC → 5 files)

**Trước (monolithic):**
```
MembersPage.tsx (1,993 LOC)
  - 16 useVueState calls
  - CRUD logic
  - Permission checks
  - Table rendering
  - Filter state
  - Multiple dialogs
```

**Sau (decomposed):**

```typescript
// MembersPage/PageNamePage.tsx (< 80 LOC) — Container only
export function MembersPage() {
  const { members, isLoading, pagination } = useMembersData();
  const { filters, setFilter } = useMembersFilters();
  const { permissions } = useMembersPermissions();
  const { createMember, updateMember, deleteMember } = useMembersActions();
  
  return (
    <MembersView
      members={members}
      isLoading={isLoading}
      filters={filters}
      permissions={permissions}
      pagination={pagination}
      onFilterChange={setFilter}
      onCreate={createMember}
      onUpdate={updateMember}
      onDelete={deleteMember}
    />
  );
}

// MembersPage/hooks/useMembersData.ts (< 100 LOC) — Data layer
export function useMembersData() {
  const members = useVueState(() => useMembersStore().memberList);
  // ... fetch logic
  return { members, isLoading, pagination };
}

// MembersPage/hooks/useMembersActions.ts (< 120 LOC) — Actions layer
export function useMembersActions() {
  // CRUD operations only
}

// MembersPage/MembersView.tsx (< 250 LOC) — Pure rendering
export function MembersView(props: MembersViewProps) {
  // JSX only, no business logic
}
```

### 2.3 Priority Refactoring Queue

| Priority | Component | LOC | Decompose Into |
|---|---|---|---|
| P0 | `MembersPage.tsx` | 1,993 | 5 files |
| P0 | `IDPsPage.tsx` | 2,104 | 6 files |
| P0 | `ProjectSyncSchemaPage.tsx` | 2,196 | 6 files |
| P1 | `DataSourceForm.tsx` | 1,997 | 4 files |
| P1 | `InstanceFormBody.tsx` | 1,913 | 4 files |
| P1 | `EnvironmentsPage.tsx` | 1,670 | 4 files |
| P1 | `IDPDetailPage.tsx` | 1,625 | 4 files |
| P2 | `ExprEditor.tsx` | 1,584 | 3 files |
| P2 | `SemanticTypesPage.tsx` | 1,431 | 3 files |
| P2 | `InstancesPage.tsx` | 1,418 | 3 files |
| P2 | `SheetTree.tsx` | 1,410 | 3 files |
| P2 | `ProjectMaskingExemptionPage.tsx` | 1,401 | 3 files |
| P3 | Remaining 6 | ~7,500 | 3 files each |

### 2.4 Hook Extraction Rules

```typescript
// Rule 1: Mỗi hook chỉ làm MỘT việc
// ❌ Bad: useMembersEverything()
// ✅ Good: useMembersData() + useMembersActions() + useMembersFilters()

// Rule 2: Hook không return JSX
// ❌ Bad: useMemberTable() { return <Table /> }
// ✅ Good: useMemberTable() { return { columns, data, onSort } }

// Rule 3: useVueState calls chỉ được phép trong data hooks, không phải containers/views
// ❌ Bad: MembersPage.tsx gọi useVueState 16 lần
// ✅ Good: useMembersData.ts gọi useVueState 1–3 lần
```

### 2.5 Lint Enforcement

Thêm vào `biome.json`:

```json
{
  "linter": {
    "rules": {
      "complexity": {
        "noExcessiveCognitiveComplexity": {
          "level": "warn",
          "options": { "maxAllowedComplexity": 15 }
        }
      }
    }
  }
}
```

Thêm custom ESLint rule (`eslint-rules/max-component-lines.mjs`):

```javascript
// Warn khi component function > 500 LOC
export const maxComponentLines = {
  create(context) {
    return {
      "FunctionDeclaration, ArrowFunctionExpression"(node) {
        const lines = node.loc.end.line - node.loc.start.line;
        if (lines > 500 && isReactComponent(node)) {
          context.report({ node, message: `Component has ${lines} lines. Max is 500. Extract hooks and sub-components.` });
        }
      }
    };
  }
};
```

---

## 3. AI-Friendly Directory Structure

Sau refactoring, AI có thể navigate codebase theo feature directory thay vì monolithic files:

```
src/react/pages/settings/members/
├── MembersPage.tsx        (~80 LOC)  ← AI entry point
├── MembersView.tsx        (~200 LOC)
├── hooks/
│   ├── useMembersData.ts  (~100 LOC)  ← AI edits HERE for data changes
│   └── useMembersActions.ts (~120 LOC) ← AI edits HERE for CRUD changes
└── components/
    ├── MembersTable.tsx   (~150 LOC)  ← AI edits HERE for table changes
    └── MembersFilters.tsx (~100 LOC)  ← AI edits HERE for filter changes
```

AI không còn cần đọc 2000 LOC — mỗi task chỉ cần đọc 1–2 files nhỏ.

---

## 4. Implementation Checklist

- [ ] Tạo decomposition template trong `.ai-context/COMPONENT_TEMPLATE.md`
- [ ] Refactor P0 components (3 files, ~6000 LOC → ~30 files)
- [ ] Refactor P1 components (5 files, ~8000 LOC → ~25 files)
- [ ] Refactor P2 components (7 files, ~10000 LOC → ~28 files)
- [ ] Thêm biome.json cognitive complexity rule
- [ ] Thêm ESLint max-component-lines rule
- [ ] Update CI: fail build nếu component > 600 LOC

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Max component LOC | 2,196 | < 500 |
| Components > 500 LOC | 18 | 0 |
| Average component LOC | ~350 | < 200 |
| AI edit precision (estimated) | ~50% | > 85% |
