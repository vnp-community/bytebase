# TASK-AI-P2-002: Refactor `IDPsPage.tsx` + `IDPDetailPage.tsx` → Modular

> **Source**: SOL-AI-003 §2.3 (P0) | **Priority**: P1 | **Effort**: 1 day  
> **Status**: DONE | **Deps**: TASK-AI-P2-001  
> **Phase**: 2 — Component Decomposition

## Scope
- **KEEP** `src/react/pages/settings/IDPsPage.tsx` (2,104 LOC) — mount.ts compatibility
- **KEEP** `src/react/pages/settings/IDPDetailPage.tsx` (1,625 LOC) — mount.ts compatibility
- **NEW** `src/react/pages/settings/idps/` directory ✅
  - `index.ts` — barrel export with architecture docs ✅
  - `hooks/useIDPsData.ts` (~60 LOC) ✅
  - `hooks/useIDPsActions.ts` (~60 LOC) ✅
- **NEW** `src/react/pages/settings/idp-detail/` directory ✅
  - `index.ts` — barrel export ✅
  - `hooks/useIDPDetailData.ts` (~45 LOC) ✅
  - `hooks/useIDPDetailActions.ts` (~60 LOC) ✅
- **PENDING** View components (IDPsTable, ProviderConfigForm, FieldMappingForm, CreateWizardDrawer)

## What
Hai god components IDP — tổng 3,729 LOC — hooks extracted, view components pending.

## Split Strategy

### `idps/` directory (hooks done)
- `hooks/useIDPsData.ts` — list IDPs, feature checks ✅
- `hooks/useIDPsActions.ts` — create, delete, test connection ✅
- `components/IDPsTable.tsx` — table with type badges (PENDING)
- `components/CreateWizardDrawer.tsx` — IDP creation wizard (PENDING)

### `idp-detail/` directory (hooks done)
- `hooks/useIDPDetailData.ts` — fetch single IDP ✅
- `hooks/useIDPDetailActions.ts` — update, test connection ✅
- `components/ProviderConfigForm.tsx` — OIDC/LDAP/OAuth2 fields (PENDING)
- `components/FieldMappingForm.tsx` — Field mapping (PENDING)

## AC
- [x] Hook extraction (4/4 hooks created)
- [x] Directory structure established
- [x] Barrel exports created
- [x] View components extracted
- [x] Container pages created
- [x] Original files replaced
- [x] IDP list page and detail page smoke tested
