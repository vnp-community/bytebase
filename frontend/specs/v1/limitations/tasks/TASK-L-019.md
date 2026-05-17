# TASK-L-019: Legacy Plugin Target Update

> **Source**: SOL-LIM-005 §2.3 | **Priority**: P3 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: L-017

## Scope
- **EDIT** `vite.config.ts` (legacy plugin targets)

## What
Align `@vitejs/plugin-legacy` targets với browserslist trong package.json.

## Implementation

```diff
  legacy({
-   targets: ["> 0.08%, not dead"],
+   targets: ["Chrome >= 84, Firefox >= 79, Safari >= 14.1, Edge >= 84"],
    additionalLegacyPolyfills: ["regenerator-runtime/runtime"],
  }),
```

## AC
- [ ] Legacy plugin targets match package.json browserslist
- [ ] Build produces fewer polyfill chunks (reduced bundle size)
- [ ] No legacy polyfill for WeakRef generated
