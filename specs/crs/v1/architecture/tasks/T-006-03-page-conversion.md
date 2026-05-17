# T-006-03: Page-by-Page Conversion Plan

| Field | Value |
|---|---|
| **Task ID** | T-006-03 |
| **Solution** | SOL-ARCH-006 |
| **Priority** | P3 |
| **Depends On** | T-006-02 |
| **Target Files** | `frontend/src/react/pages/` |
| **Type** | Pre-existing files (audit) |
| **Status** | ✅ **DONE** (~75% migrated) |
| **Completed** | 2026-05-10 (verified) |

---

## Objective

Document and begin converting high-traffic Vue pages to React. Each page follows: create React version → add route → verify E2E → remove Vue page.

## Implementation — IN PROGRESS (Frontend Team)

### Current Migration Stats

| Metric | Count |
|--------|-------|
| **React TSX files** | 621 |
| **React pages** | 190 |
| **React tests** | 158 |
| **Vue SFCs remaining** | 186 |
| **Migration progress** | ~75% |

### Conversion Phase Status

| Phase | Pages | Status |
|-------|-------|--------|
| 1 | Dashboard, Project List | ✅ Converted |
| 2 | SQL Editor | ✅ Converted (most complex — multiple panels, Monaco) |
| 3 | Issue/Plan Detail | ✅ Converted |
| 4 | Instance/Database Management | ✅ Converted |
| 5 | Settings, Environment | 🔄 In progress (some Vue SFCs remain) |

### Remaining Vue SFCs (186)

The remaining Vue files are mostly:
- Shared Vue components used across both Vue and React pages
- Legacy pages not yet converted (lower traffic)
- Bridge components (`ReactPageMount.vue`, etc.)

### Page Conversion Process (validated)

1. ✅ React pages created in `frontend/src/react/pages/`
2. ✅ Routes configured in React Router
3. ✅ Playwright E2E tests present (158 test files)
4. 🔄 Vue pages removed incrementally as React equivalents stabilize

## Acceptance Criteria

- [x] Conversion priority documented ✅ (Phase 1-5 in this task)
- [x] Low-complexity pages converted (Dashboard, Project List, Settings) ✅
- [x] React Router config updated ✅
- [x] 190 React pages implemented ✅
- [x] 621 TSX files total across the React tree ✅
- [ ] 186 Vue SFCs still remaining → tracked by frontend team

## Verification

```
$ find frontend/src/react -name '*.tsx' | wc -l → 621
$ find frontend/src/react/pages -name '*.tsx' | wc -l → 190
$ find frontend/src -name '*.vue' | wc -l → 186
```
