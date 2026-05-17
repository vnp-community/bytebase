# TASK-AI-P1-006: Tạo Per-Module `.ai-context.md` — Settings + Project + Auth

> **Source**: SOL-AI-005 §2.3 | **Priority**: P1 | **Effort**: 3h  
> **Status**: ✅ DONE | **Deps**: TASK-AI-P0-001  
> **Phase**: 1 — Tooling & Lint

## Scope
- **NEW** `src/react/pages/settings/.ai-context.md`
- **NEW** `src/react/pages/project/.ai-context.md`
- **NEW** `src/react/pages/auth/.ai-context.md`
- **NEW** `src/react/components/.ai-context.md`
- **NEW** `src/store/.ai-context.md`

## What
Per-module context files để AI biết chính xác scope, primary files, và dependencies của mỗi module — không cần đọc cả directory.

## Implementation

### `src/react/pages/settings/.ai-context.md`
```markdown
# Module: Settings Pages

## Scope
Workspace-level settings: members, users, groups, roles, IDPs, environments,
instances, SQL review, subscription, audit log, general settings.

## Primary Files (đọc file này khi task trong settings module)
- `MembersPage.tsx` (refactored) — member management
- `UsersPage.tsx` — user CRUD
- `GroupsPage.tsx` — group management
- `EnvironmentsPage.tsx` (refactored) — environment ordering
- `InstancesPage.tsx` (refactored) — instance list
- `IDPsPage.tsx` (refactored) — identity provider
- `SettingGeneralPage.tsx` — workspace settings

## Dependencies
- API Clients: userServiceClient, instanceServiceClient, settingServiceClient
- Stores: useAuthStore (permission), useSubscriptionStore (feature gating)
- Permissions: workspace-level "bb.users.*", "bb.instances.*", "bb.settings.*"

## Common Tasks → File to Edit
- Add member filter → `hooks/useMembersFilters.ts`
- Add instance column → `InstancesPage/components/InstancesTable.tsx`
- Add IDP field → `IDPDetailPage/components/IDPForm.tsx`

## Prohibited
- No useVueState calls (migrated to TanStack Query in Phase 3)
- No raw z-index
- Use semantic tokens
```

### `src/react/pages/project/.ai-context.md`
Similar structure for: project databases, plans, issues, members, settings, masking, schema sync, branches.

### `src/react/pages/auth/.ai-context.md`
Scope: signin, MFA, SSO callbacks, password reset.
Warning: OAuth callback routes bypass navigation guards — do NOT add guards here.

### `src/react/components/.ai-context.md`
Shared components inventory: what exists (don't recreate), UI primitives at `components/ui/`, shadcn-style pattern.

### `src/store/.ai-context.md`
Store domain mapping: domain → Pinia store file → migration status (migrated/pending TanStack Query).
Warning: Legacy stores being phased out — prefer TanStack Query hooks in React code.

## AC
- [ ] 5 `.ai-context.md` files tạo xong theo template format
- [ ] Mỗi file có: Scope, Primary Files, Dependencies, Common Tasks, Prohibited
- [ ] File paths trong Primary Files là chính xác
- [ ] Permissions strings khớp với PermissionStore values
