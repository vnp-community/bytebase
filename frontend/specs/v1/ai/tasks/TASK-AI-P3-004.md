# TASK-AI-P3-004: Tạo Zustand Stores — Auth + UI + SQLEditor

> **Source**: SOL-AI-004 §2.3 | **Priority**: P1 | **Effort**: 4h  
> **Status**: DONE | **Deps**: TASK-AI-P3-001  
> **Phase**: 3 — State Architecture Migration

## Scope
- **NEW** `src/react/stores/auth.ts` ✅
- **NEW** `src/react/stores/ui.ts` ✅
- **NEW** `src/react/stores/sqlEditor.ts` ✅
- **EDIT** `src/react/stores/index.ts` ✅

## What Done

### `auth.ts` (85 LOC) ✅
- State: currentUser, isLoggedIn, requireResetPassword, requireMfa
- Actions: setCurrentUser, clearAuth, setRequireResetPassword, setRequireMfa
- Middleware: `devtools` (named "bb-auth" for Zustand DevTools)
- Bridge: `syncAuthFromVue()` — watches Pinia store and syncs to Zustand during transition
- Watch function injected as parameter to avoid importing Vue directly in React code

### `ui.ts` (53 LOC) ✅
- State: locale, theme, sidebarCollapsed, quickstartDismissed
- Actions: setLocale, setTheme, toggleSidebar, dismissQuickstart
- Middleware: `devtools` + `persist` (localStorage key "bb-ui")
- Persists locale/theme across page reload

### `sqlEditor.ts` (96 LOC) ✅
- State: activeTabId, tabs[], isExecuting
- Actions: addTab, removeTab, setActiveTab, updateTabStatement, updateTabConnection, setIsExecuting, clearTabs
- Middleware: `devtools` only (NOT persisted — session-only)
- Auto-selects last tab when active tab is removed

### `index.ts` barrel ✅
- Re-exports existing `stores/app/` (createAppStore, AppStoreState)
- Exports new standalone stores (useAuthStore, useUIStore, useSQLEditorStore)
- Clear documentation separating server-state (TanStack Query) from client-state (Zustand)

## Architecture Note
Existing `stores/app/` uses a combined Zustand store pattern (slices).
New stores are standalone to support gradual migration — they can be consumed
independently by React-only components without pulling in the full app store.

## AC
- [x] 3 stores tạo xong
- [x] `syncAuthFromVue()` utility exported
- [x] TypeScript compiles (0 new errors)
- [x] useAuthStore.currentUser accessible từ React components
- [x] UIStore persists locale/theme qua page reload (localStorage "bb-ui")
