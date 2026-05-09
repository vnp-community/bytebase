# ARCH-LIM-006 — Dual Frontend Framework (Vue 3 + React 19)

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Limitation (Migration Trade-off)           |
| **Layer**      | L1 (Presentation)                          |
| **Impact**     | Bundle Size, DX Complexity, Build Time     |
| **Severity**   | Medium                                     |

---

## 1. Description

Frontend chạy đồng thời **Vue 3.5** (legacy) và **React 19** (new code) trong cùng một SPA, với bridge layer `useVueState()` để React subscribe vào Vue/Pinia reactive state.

### Evidence (architecture.md §2 + TDD.md §10)

```
frontend/src/
  ├── Vue 3 Layer (legacy)
  │     ├── components/     (Vue SFC)
  │     ├── composables/    (Vue composition API)
  │     ├── store/modules/  (Pinia stores)
  │     └── views/          (Vue pages)
  │
  ├── React 19 Layer (new)
  │     ├── components/ui/  (shadcn-style)
  │     ├── hooks/          (React hooks)
  │     └── pages/          (React pages)
  │
  └── Bridge Layer
        └── useVueState(getter)  ← React subscribes to Vue state
```

### Technology Stack Overlap

| Concern | Vue Layer | React Layer |
|---------|-----------|-------------|
| Component lib | Naive UI | Base UI (shadcn) |
| State | Pinia 3 | Zustand |
| Router | Vue Router 4 | (embedded in Vue Router) |
| i18n | vue-i18n | react-i18next |
| Build | Vite 7.3 | Vite 7.3 (shared) |
| Styling | Tailwind v4 | Tailwind v4 (shared) |

---

## 2. Consequences

| Consequence | Description |
|------------|-------------|
| **Bundle Size** | Two runtime libraries (Vue + React) → 30-50KB additional gzipped |
| **Bridge Complexity** | `useVueState` creates implicit coupling between frameworks |
| **Double Learning Curve** | Developers must know both Vue and React patterns |
| **Testing Gap** | Two different testing patterns (Vitest+vue-test-utils vs Jest+RTL) |
| **Style Conflicts** | Two component libraries with different design tokens |
| **Migration Uncertainty** | Unclear timeline for complete Vue→React migration |

---

## 3. Root Cause

Vue 3 was original choice. React 19 migration started for:
- Better TypeScript support
- Larger ecosystem (shadcn, radix)
- Modern patterns (Server Components, Suspense)

Migration is gradual — cannot rewrite entire frontend at once.
