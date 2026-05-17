# TASK-W-025: Unified Title Manager

> **Source**: SOL-WEAK-008 §2.5 | **Priority**: P3 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **NEW** `src/utils/title-manager.ts`
- **EDIT** `src/router/index.ts` (afterEach hook)

## What
Create `setPageTitle(title, source)` with source tracking (vue/react) and debounce to prevent flicker.

## Implementation — see SOL-WEAK-008 §2.5
- `setPageTitle(title, "vue"|"react")`: React priority over Vue, 50ms debounce
- `resetTitleTracking()`: called in `beforeEach`
- Router `afterEach`: use `setPageTitle(title, "vue")` instead of `setDocumentTitle`

## AC
- [x] `title-manager.ts` created
- [x] No title flicker on React page navigation
- [x] React title takes priority when both set
- [x] Title resets on navigation start
