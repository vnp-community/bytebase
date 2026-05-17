# TASK-W-020: Replace Empty Catches

> **Source**: SOL-WEAK-005 §2.4 | **Priority**: P3 | **Effort**: 2h  
> **Status**: DONE | **Deps**: —

## Scope (6 files)
1. `src/store/modules/v1/auth.ts` L202, L225
2. `src/store/modules/sqlEditor/worksheet.ts` L382
3. `src/utils/web-storage.ts` L39, L58
4. `src/plugins/ai/store/conversation.ts` L276
5. `src/views/sql-editor/EditorCommon/ResultView/SingleResultViewV1.vue` L742

## What
Replace `catch { /* nothing */ }` with structured `console.warn` logging using 3 patterns:
- **Pattern 1** (expected errors): discriminate by error code, log non-expected
- **Pattern 2** (persistence): `console.warn("[Storage]", key, error)`
- **Pattern 3** (best-effort): `console.warn("[Module]", error)`

## Implementation
Each catch block → add `console.warn("[ModuleName] description:", error)`. See SOL-WEAK-005 §2.4 for file-by-file mapping.

## AC
- [x] Zero empty catch blocks in listed files
- [x] Each catch has descriptive `console.warn` with module tag
- [x] No behavioral changes (still gracefully degrade)
