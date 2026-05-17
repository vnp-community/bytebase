# TASK-AI-P3-001: Cài đặt TanStack Query + Setup QueryProvider

> **Source**: SOL-AI-004 §2.2 | **Priority**: P1 | **Effort**: 4h  
> **Status**: DONE | **Deps**: —  
> **Phase**: 3 — State Architecture Migration

## Scope
- **EDIT** `package.json` — add `@tanstack/react-query` v5 ✅
- **NEW** `src/react/providers/QueryProvider.tsx` ✅
- **NEW** `src/react/hooks/queries/query-keys.ts` ✅
- **EDIT** `src/react/mount.ts` — wrap với QueryProvider ✅

## What Done

### 1. Dependencies installed ✅
- `@tanstack/react-query@5.100.10` (production)
- `@tanstack/react-query-devtools@5` (dev only)

### 2. QueryProvider ✅
- ConnectRPC-aware retry logic (instant bail on NotFound, PermissionDenied, Unauthenticated, etc.)
- DevTools only in dev mode (import.meta.env.DEV)
- Default staleTime 5 min, gcTime 30 min, refetchOnWindowFocus disabled

### 3. Query Keys ✅
- All 24 domains covered with hierarchical key factory
- Supports `.all` (invalidate everything), `.list(parent)`, `.detail(name)` patterns
- Special keys: `project.iamPolicy`, `schema.metadata`, `subscription.current`

### 4. Mount.ts Integration ✅
- QueryProvider lazily loaded alongside React/i18n deps
- Wraps StrictMode → QueryProvider → I18nextProvider → Component
- All existing `mountReactPage` and `updateReactPage` calls automatically get TanStack Query context

## AC
- [x] `@tanstack/react-query` v5 installed
- [x] `QueryProvider` wraps root in `mount.ts`
- [x] `query-keys.ts` defines keys cho tất cả 24 domains
- [x] React Query Devtools visible in dev mode
- [x] `tsc --noEmit --project tsconfig.react.json` pass (0 new errors)
- [x] No existing functionality broken
