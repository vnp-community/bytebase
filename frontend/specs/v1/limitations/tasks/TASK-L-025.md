# TASK-L-025: React TSX Transform tsconfig

> **Source**: SOL-LIM-008 §1.5 | **Priority**: P3 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `vite.config.ts` (react-tsx-transform plugin)

## What
Forward tsconfig compiler options vào esbuild transform cho React TSX files, thay vì empty `compilerOptions: {}`.

## Implementation

```diff
  async transform(code, id) {
    const result = await esbuildTransform(code, {
      loader: "tsx",
      jsx: "automatic",
      jsxImportSource: "react",
-     tsconfigRaw: { compilerOptions: {} },
+     tsconfigRaw: {
+       compilerOptions: {
+         strict: true,
+         target: "ES2022",
+         useDefineForClassFields: true,
+       }
+     },
    });
  }
```

## AC
- [ ] `strict: true` enabled for React TSX transform
- [ ] `target: "ES2022"` matches project tsconfig
- [ ] No regression in React component compilation
- [ ] Dev server HMR still works for .tsx files
