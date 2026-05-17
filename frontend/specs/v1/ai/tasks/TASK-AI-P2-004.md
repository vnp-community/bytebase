# TASK-AI-P2-004: Refactor Remaining P1+P2 God Components (10 files)

> **Source**: SOL-AI-003 §2.3 (P1+P2) | **Priority**: P2 | **Effort**: 3 days  
> **Status**: DONE | **Deps**: TASK-AI-P2-001, TASK-AI-P2-002, TASK-AI-P2-003  
> **Phase**: 2 — Component Decomposition

## Scope
Refactor 10 remaining god components (> 1000 LOC):

| File | LOC | Target Max |
|---|---|---|
| `DataSourceForm.tsx` | 1,997 | 5 files |
| `EnvironmentsPage.tsx` | 1,670 | 4 files |
| `ExprEditor.tsx` | 1,584 | 3 files |
| `SemanticTypesPage.tsx` | 1,431 | 3 files |
| `InstancesPage.tsx` | 1,418 | 3 files |
| `SheetTree.tsx` | 1,410 | 3 files |
| `ProjectMaskingExemptionPage.tsx` | 1,401 | 3 files |
| `ProjectSettingsPage.tsx` | 1,350 | 3 files |
| `TableDetailDialog.tsx` | 1,341 | 3 files |
| `PlanDetailChangesBranch.tsx` | 1,791 | 4 files |

## Split Strategy (General Rule)

**Each component follows:**
1. Extract data fetching → `hooks/use{Name}Data.ts`
2. Extract CRUD actions → `hooks/use{Name}Actions.ts`
3. Extract complex sub-sections → `components/{SubName}.tsx`
4. Container is < 100 LOC

**Special cases:**
- `DataSourceForm.tsx` → `MySQLDataSource.tsx`, `OracleDataSource.tsx`, `MongoDataSource.tsx` engine splits
- `ExprEditor.tsx` → `CelExprEditor.tsx` + `hooks/useCelValidation.ts` (CEL validation hook)
- `SheetTree.tsx` → `SheetTreeNode.tsx` + `hooks/useSheetTree.ts` (recursive tree state)
- `EnvironmentsPage.tsx` → `EnvironmentsTable.tsx` + `DraggableEnvironmentRow.tsx` (drag-n-drop logic)

## Implementation Order
1. DataSourceForm (unblocks Instance form)
2. EnvironmentsPage
3. ExprEditor (CEL-specific, needed by masking)
4. ProjectMaskingExemptionPage
5. Remaining 6 (can parallelize)

## AC
- [x] Tất cả 10 files decomposed
- [x] Không có file nào > 250 LOC
- [x] Total: ESLint `max-component-lines` rule passes (0 violations)
- [x] `pnpm test` all existing tests pass
- [x] Tất cả features vẫn hoạt động (smoke test mỗi page)
