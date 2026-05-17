# TASK-W-034: Migrate High-Traffic localStorage

> **Source**: SOL-WEAK-007 §2.4 | **Priority**: P3 | **Effort**: 5h  
> **Status**: DONE | **Deps**: W-031

## Scope
- **EDIT** 10+ files using raw `localStorage.getItem`/`setItem`

## What
Replace high-traffic raw localStorage calls with `storageService` API. Priority files:
1. `src/utils/web-storage.ts` — `WebStorageHelper` class
2. `src/store/modules/sqlEditor/tab.ts` — tab persistence
3. `src/store/modules/sqlEditor/uiState.ts` — UI preferences
4. `src/store/modules/sqlEditor/queryHistory.ts` — query log
5. `src/composables/*.ts` — various composables using localStorage
6. `src/react/stores/app/*.ts` — React app state persistence

## Approach
Each file: replace `localStorage.getItem(key)` → `storageService.load(key, { namespace }, fallback)` and `localStorage.setItem(key, value)` → `storageService.save(key, value, { namespace })`.

## AC
- [x] Zero raw `localStorage.getItem/setItem` calls in listed files
- [x] All persistence uses namespaced `storageService`
- [x] Existing user data still accessible (backward compatible read)
- [x] Quota monitoring active via `storageService.getStats()`
