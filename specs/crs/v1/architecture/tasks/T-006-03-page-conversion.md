# T-006-03: Page-by-Page Conversion Plan

| Field | Value |
|---|---|
| **Task ID** | T-006-03 |
| **Solution** | SOL-ARCH-006 |
| **Priority** | P3 |
| **Depends On** | T-006-02 |
| **Target Files** | `frontend/src/react/pages/` |
| **Type** | New files |

---

## Objective

Document and begin converting high-traffic Vue pages to React. Each page follows: create React version → add route → verify E2E → remove Vue page.

## Page Conversion Priority

| Phase | Pages | Complexity |
|-------|-------|------------|
| 1 | Dashboard, Project List | Low |
| 2 | SQL Editor | High |
| 3 | Issue/Plan Detail | High |
| 4 | Instance/Database Management | Medium |
| 5 | Settings, Environment | Low |

## Per-Page Process

1. Create `frontend/src/react/pages/ProjectList.tsx`
2. Add route in React Router
3. Run Playwright E2E to verify feature parity
4. Remove Vue page + vue-router entry
5. Track bundle size delta

## Acceptance Criteria

- [ ] Conversion priority documented
- [ ] At least 2 low-complexity pages converted (Dashboard, Project List)
- [ ] React Router config updated
- [ ] `useVueState` bridge not used in converted pages
