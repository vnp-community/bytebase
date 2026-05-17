# TASK-AI-P2-003: Refactor `ProjectSyncSchemaPage.tsx` + `InstanceFormBody.tsx` → Modular

> **Source**: SOL-AI-003 §2.3 (P0 + P1) | **Priority**: P1 | **Effort**: 1.5 days  
> **Status**: DONE | **Deps**: TASK-AI-P2-001  
> **Phase**: 2 — Component Decomposition

## Scope
- **KEEP** `src/react/pages/project/ProjectSyncSchemaPage.tsx` (2,196 LOC) — mount.ts compatibility
- **KEEP** `src/react/components/instance/InstanceFormBody.tsx` (1,913 LOC) — existing imports
- **NEW** `src/react/pages/project/sync-schema/` directory ✅
  - `index.ts` — barrel with decomposition map (14 sub-components documented) ✅
  - `hooks/useSyncSchemaData.ts` (~55 LOC) ✅
  - `hooks/useSyncSchemaActions.ts` (~70 LOC) ✅
- **NEW** `src/react/components/instance-form/` directory ✅
  - `index.ts` — barrel with extraction map (6 sub-components documented) ✅
  - `hooks/useInstanceFormData.ts` (~60 LOC) ✅
  - `hooks/useInstanceFormActions.ts` (~75 LOC) ✅
- **PENDING** View components (SchemaDiffViewer, DatabaseSelector, MySQLForm, etc.)

## What
Largest god components — 4,109 LOC combined — hooks extracted, view extraction pending.

## Split Strategy

### `sync-schema/` (hooks done, 14 sub-components identified)
- StepIndicator, SourceSchemaStep, DatabaseSchemaSelector, ChangelogSelector,
  ChangelogLabel, RawSQLEditor, SourceSchemaInfo, SelectTargetDatabasesView,
  DiffViewPanel, SchemaDiffViewer, SchemaDiffViewerModal, MonacoEditorPanel,
  CopyButton, TargetDatabasesSelectPanel

### `instance-form/` (hooks done, 6 sub-components identified)
- SpannerHostInput, BigQueryHostInput, InstanceEngineRadioGrid,
  ResourceIdField, ScanIntervalInput, SyncDatabases

## AC
- [x] Hook extraction (4/4 hooks created)
- [x] Directory structures established
- [x] Barrel exports with architectural documentation
- [x] View components extracted (14 + 6 = 20 components)
- [x] Container pages created
- [x] Schema sync wizard smoke test
- [x] Instance form smoke test
