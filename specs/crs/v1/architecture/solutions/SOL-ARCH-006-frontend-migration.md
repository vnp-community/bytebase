# Solution: Frontend Migration Acceleration — CR-ARCH-006

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-006                                             |
| **CR Reference**   | CR-ARCH-006                                              |
| **Title**          | Incremental Vue→React Migration via Module Federation    |
| **Affected Layers**| L1 (Presentation)                                        |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §1 (L1 — Presentation):
- Vue 3.5 (Naive UI, Pinia 3) + React 19 (Base UI, Zustand 5)
- Bridge: `useVueState()` — React subscribes to Vue/Pinia reactive state
- Build: Vite 7.3, shared Tailwind v4

Per [TDD.md](../../TDD.md) §10 (Frontend Architecture):
- Vue Router 4 manages all routes
- React pages embedded within Vue layout via bridge
- Dual i18n: vue-i18n + react-i18next

---

## 2. Solution Design

### 2.1 Migration Strategy: Page-by-Page Conversion

```
Phase 1: React Shell (Layout, Nav, Auth)
  → All shared chrome becomes React
  → Vue pages rendered inside React layout wrapper

Phase 2: High-Traffic Pages (SQL Editor, Issue/Plan)
  → Page-by-page React rewrite
  → Each converted page uses Zustand directly (no bridge)

Phase 3: Management Pages (Project, Instance, Settings)
  → Lower-priority CRUD pages
  → Simpler conversion (mostly forms/tables)

Phase 4: Vue Removal
  → Remove Vue runtime, Pinia, vue-i18n, Naive UI
  → React Router replaces Vue Router
```

### 2.2 State Migration Pattern

**Bridge during transition** (existing `useVueState`):

```typescript
// Current bridge — React reads Vue state
export function useVueState<T>(getter: () => T): T {
  const [state, setState] = useState(getter());
  useEffect(() => {
    const unwatch = watch(getter, (val) => setState(val));
    return unwatch;
  }, []);
  return state;
}
```

**Target — pure Zustand store**:

```typescript
// New Zustand store mirrors Pinia store
import { create } from 'zustand';

interface ProjectState {
  currentProject: Project | null;
  projects: Project[];
  fetchProjects: () => Promise<void>;
  setCurrentProject: (p: Project) => void;
}

export const useProjectStore = create<ProjectState>((set) => ({
  currentProject: null,
  projects: [],
  fetchProjects: async () => {
    const resp = await projectService.listProjects();
    set({ projects: resp.projects });
  },
  setCurrentProject: (p) => set({ currentProject: p }),
}));
```

### 2.3 Component Mapping (Naive UI → Base UI/Radix)

| Naive UI | Base UI/Radix Equivalent | Complexity |
|----------|------------------------|------------|
| `NButton` | `Button` (Base UI) | Low |
| `NModal` | `Dialog` (Radix) | Medium |
| `NDataTable` | `Table` (custom + TanStack) | High |
| `NForm` / `NFormItem` | `Form` (React Hook Form) | Medium |
| `NSelect` | `Select` (Radix) | Medium |
| `NDropdown` | `DropdownMenu` (Radix) | Low |
| `NTabs` | `Tabs` (Radix) | Low |
| `NTooltip` | `Tooltip` (Radix) | Low |
| `NTree` | `Tree` (custom) | High |
| `NMessage` / `NNotification` | `Toast` (Sonner) | Low |

### 2.4 i18n Consolidation

```typescript
// Unified i18n via react-i18next
// Import existing translation JSON files (shared between Vue and React)
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import enUS from '../locales/en-US.json';
import zhCN from '../locales/zh-CN.json';

i18n.use(initReactI18next).init({
  resources: { en: { translation: enUS }, zh: { translation: zhCN } },
  lng: 'en',
  fallbackLng: 'en',
});
```

---

## 3. File Change Manifest

| Component | Action | Files |
|-----------|--------|-------|
| React Shell (Layout) | **NEW** | `frontend/src/react/layout/AppLayout.tsx` |
| Zustand Stores | **NEW** | `frontend/src/react/stores/*.ts` |
| Base UI Components | **NEW** | `frontend/src/react/components/ui/*.tsx` |
| Page Conversion | **MODIFY** | Per-page `.vue` → `.tsx` |
| Vue Bridge | **DEPRECATE** | `frontend/src/react/useVueState.ts` |
| Vite Config | **MODIFY** | Tree-shake Vue deps incrementally |

---

## 4. Migration Strategy

Each page migration follows:
1. Create React version in `frontend/src/react/pages/`
2. Add route in React Router
3. Verify feature parity with Playwright E2E
4. Remove Vue page + route
5. Track bundle size delta

### CI Enforcement

```bash
# Track migration progress
echo "Vue SFC files: $(find src -name '*.vue' | wc -l)"
echo "React TSX files: $(find src/react -name '*.tsx' | wc -l)"
echo "useVueState imports: $(grep -r 'useVueState' src/react | wc -l)"
```

---

## 5. Rollback Plan

Each page migration is independent — revert a single page by restoring Vue route.
