# TASK-AI-P1-001: Tạo `src/types/ai-ref/` — Service Map + Core Domain Types

> **Source**: SOL-AI-002 §2.1-2.2 | **Priority**: P1 | **Effort**: 4h  
> **Status**: ✅ DONE | **Deps**: —  
> **Phase**: 1 — Tooling & Lint

## Scope
- **NEW** `src/types/ai-ref/service-map.ts`
- **NEW** `src/types/ai-ref/database.ts`
- **NEW** `src/types/ai-ref/project.ts`
- **NEW** `src/types/ai-ref/instance.ts`
- **NEW** `src/types/ai-ref/index.ts`

## What
Tạo condensed AI reference layer — 5 files ~500 LOC thay thế 38K LOC proto-es cho AI consumption.

## Implementation

### `service-map.ts`
```typescript
/**
 * AI SERVICE MAP — Domain → Client → Store lookup.
 * Source of truth for AI code generation.
 */
export const SERVICE_MAP = {
  database: {
    client: "databaseServiceClientConnect",
    store: "useDatabaseV1Store",          // Legacy Pinia (use TanStack Query if available)
    queryHook: "useDatabase",              // TanStack Query hook (preferred)
    protoFile: "database_service_pb",
    updateMaskFields: ["labels", "environment", "project", "title"],
  },
  project: { client: "projectServiceClientConnect", store: "useProjectV1Store", queryHook: "useProject", ... },
  instance: { client: "instanceServiceClientConnect", store: "useInstanceV1Store", queryHook: "useInstance", ... },
  issue: { client: "issueServiceClientConnect", store: "useIssueV1Store", queryHook: "useIssue", ... },
  plan: { client: "planServiceClientConnect", store: "usePlanStore", queryHook: "usePlan", ... },
  rollout: { client: "rolloutServiceClientConnect", store: "useRolloutStore", queryHook: "useRollout", ... },
  user: { client: "userServiceClientConnect", store: "useUserStore", queryHook: "useUser", ... },
  auth: { client: "authServiceClientConnect", store: "useAuthStore", ... },
  setting: { client: "settingServiceClientConnect", store: "useSettingV1Store", ... },
  sql: { client: "sqlServiceClientConnect", store: null, ... },
  // ... 14 more entries
} as const;
```

### `database.ts` — Condensed DatabaseRef interface
Header comment: "AI REFERENCE — Full type at src/types/proto-es/v1/database_service_pb.d.ts"
- Interface `DatabaseRef` với 8 top-level fields (name, title, environment, instance, project, labels, lastSuccessfulSyncTime, schemaSize)
- `DATABASE_CLIENT` constant
- `DATABASE_UPDATE_MASK_FIELDS` constant
- JSDoc service client methods: get, list, update, search

### `project.ts`, `instance.ts` — Same pattern

### `index.ts` — Re-export all

## AC
- [ ] `service-map.ts` có đủ 24+ service entries
- [ ] Mỗi domain type file có: condensed interface, client constant, update mask fields, JSDoc methods
- [ ] TypeScript compiles (`pnpm tsc --noEmit`)
- [ ] Tổng file size < 200KB
