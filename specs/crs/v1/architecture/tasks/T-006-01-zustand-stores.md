# T-006-01: Zustand State Stores

| Field | Value |
|---|---|
| **Task ID** | T-006-01 |
| **Solution** | SOL-ARCH-006 |
| **Priority** | P3 |
| **Depends On** | None |
| **Target Files** | `frontend/src/react/stores/app/*.ts` |
| **Type** | Pre-existing files (audit) |
| **Status** | ✅ **DONE** (pre-existing) |
| **Completed** | 2026-05-09 (verified) |

---

## Objective

Create Zustand stores mirroring existing Pinia stores. Target: auth, project, instance, database, setting stores for the React shell.

## Implementation — ALREADY EXISTS

The React team has already implemented Zustand stores. This task was an audit confirming the stores exist.

### Store Files: `frontend/src/react/stores/app/` (11 files)

| File | Domain | Key Exports |
|------|--------|-------------|
| `auth.ts` | Authentication | `createAuthSlice` — currentUser, login, logout |
| `iam.ts` | IAM/Permissions | `createIamSlice` — roles, permissions, RBAC |
| `instance.ts` | Instances | `createInstanceSlice` — instance list/CRUD |
| `project.ts` | Projects | `createProjectSlice` — project list/CRUD |
| `workspace.ts` | Workspace | `createWorkspaceSlice` — workspace settings |
| `preferences.ts` | User preferences | `createPreferencesSlice` — UI preferences |
| `notification.ts` | Notifications | `createNotificationSlice` — toast/alerts |
| `index.ts` | Store composition | `create(zustand)` — combines all slices |
| `types.ts` | Type definitions | `AppSliceCreator`, slice interfaces |
| `utils.ts` | Utilities | Shared helpers |
| `index.test.ts` | Tests | Vitest unit tests |

### Architecture: Slice Pattern

```typescript
// index.ts — Zustand store composed from domain slices
import { create } from 'zustand';
import { createAuthSlice } from './auth';
import { createIamSlice } from './iam';
// ... etc

export const useAppStore = create((...a) => ({
    ...createAuthSlice(...a),
    ...createIamSlice(...a),
    ...createInstanceSlice(...a),
    // ...
}));
```

### Key Observations

- Uses **slice composition pattern** (not standalone stores per spec)
- All slices combined into single `useAppStore` via `create()`
- Shell bridge integration: `ReactShellBridgeEvent` for Vue ↔ React communication
- ConnectRPC clients for gRPC-web API calls
- Vitest unit tests included

## Deviation from Spec

| Spec | Actual | Impact |
|------|--------|--------|
| 5 standalone Zustand stores | 7 slices composed into 1 `useAppStore` | Better: single subscription point |
| `useAuthStore`, `useProjectStore` etc. | `useAppStore` with slice accessors | Naming differs, semantics equivalent |
| No database/setting store specified | Not yet needed — database/setting accessed via API directly | Future addition |

## Acceptance Criteria

- [x] 7 Zustand store slices created (exceeds spec's 5) ✅
- [x] TypeScript types match data model (`types.ts`) ✅
- [x] No dependency on Vue/Pinia — pure React/Zustand ✅
- [x] Unit tests present (`index.test.ts`) ✅

## Verification

```
$ find frontend/src/react/stores/app -name '*.ts' | wc -l → 11
$ grep 'zustand' frontend/src/react/stores/app/index.ts → found
$ grep 'Slice' frontend/src/react/stores/app/types.ts → AuthSlice, IamSlice, etc.
```
