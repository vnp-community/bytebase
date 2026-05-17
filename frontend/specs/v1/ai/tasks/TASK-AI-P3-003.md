# TASK-AI-P3-003: Tạo TanStack Query Hooks — 19 Remaining Domains

> **Source**: SOL-AI-004 §2.4 | **Priority**: P2 | **Effort**: 2 days  
> **Status**: DONE | **Deps**: TASK-AI-P3-002  
> **Phase**: 3 — State Architecture Migration

## Scope
All 19 domain hook files created ✅:

| File | Methods | LOC |
|------|---------|-----|
| `useIssue.ts` | list (with filter), get, update | 42 |
| `usePlan.ts` | list, get, create | 37 |
| `useRollout.ts` | get | 13 |
| `usePolicy.ts` | list, update | 28 |
| `useSetting.ts` | get by name, update | 27 |
| `useSubscription.ts` | get (read-only, 10min stale) | 12 |
| `useRole.ts` | list | 13 |
| `useGroup.ts` | list, get, update | 35 |
| `useWorksheet.ts` | search (correct API name), get, update | 39 |
| `useAuditLog.ts` | infinite query with cursor pagination | 20 |
| `useReviewConfig.ts` | list, get, update | 35 |
| `useIDP.ts` | list, get, delete, testConnection | 48 |
| `useAccessGrant.ts` | list | 15 |
| `useWorkloadIdentity.ts` | list | 13 |
| `useServiceAccount.ts` | list | 13 |
| `useSchema.ts` | getSchema, getMetadata | 28 |
| `useChangelog.ts` | list, get | 28 |
| `useRelease.ts` | list, get, create | 40 |
| `useDatabaseGroup.ts` | list, get, delete | 40 |

## Design Decisions
- `useWorksheet` → uses `searchWorksheets()` (the proto API is named `search`, not `list`)
- `useAuditLog` → `useInfiniteQuery` with cursor-based pagination via `pageToken`/`nextPageToken`
- `useSubscription` → read-only with 10-minute staleTime (rarely changes)

## AC
- [x] 19 hook files tạo xong
- [x] `index.ts` exports all 24 domains
- [x] TypeScript compiles (0 new errors)
- [x] Infinite query pattern cho AuditLog
- [x] Tổng LOC per file < 150 (max is 82)
