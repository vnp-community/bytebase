# TASK-AI-P3-002: Tạo TanStack Query Hooks — 5 Core Domains

> **Source**: SOL-AI-004 §2.2 | **Priority**: P1 | **Effort**: 1 day  
> **Status**: DONE | **Deps**: TASK-AI-P3-001  
> **Phase**: 3 — State Architecture Migration

## Scope
- **NEW** `src/react/hooks/queries/useDatabase.ts` ✅
- **NEW** `src/react/hooks/queries/useProject.ts` ✅
- **NEW** `src/react/hooks/queries/useInstance.ts` ✅
- **NEW** `src/react/hooks/queries/useUser.ts` ✅
- **NEW** `src/react/hooks/queries/useEnvironment.ts` ✅
- **NEW** `src/react/hooks/queries/index.ts` (barrel) ✅

## What Done
5 core domain query hooks with full CRUD coverage:

| Domain | Hooks | LOC |
|--------|-------|-----|
| Database | get, list, update, batchUpdate | 75 |
| Project | get, list, update, delete, iamPolicy | 75 |
| Instance | get, list, update, create, delete | 82 |
| User | get, list, update, delete | 63 |
| Environment | getSetting (environments stored as setting) | 24 |

## Design Decisions
- `useEnvironment` → uses `settingServiceClientConnect.getSetting()` because environments are stored as a workspace setting in Bytebase (not a separate Environment service)
- `useInstance.listInstances` → no `parent` parameter (uses `filter` instead per proto)
- Cache invalidation via `queryKeys.<domain>.all` on mutations
- All hooks use `enabled: !!name` to prevent queries with empty keys

## AC
- [x] 5 domain hook files tạo xong
- [x] Mỗi file: get, list, update (create và delete nếu applicable)
- [x] `select` transform để unwrap responses
- [x] Cache invalidation sau mutations
- [x] TypeScript compiles (tsconfig.react.json — 0 new errors)
