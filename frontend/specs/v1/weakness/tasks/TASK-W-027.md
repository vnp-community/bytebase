# TASK-W-027: Strip Console.debug in Build

> **Source**: SOL-WEAK-006 §2.3 | **Priority**: P3 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `vite.config.ts`

## What
Add `esbuild.pure` config to strip `console.debug` in production builds.

## Implementation
```diff
 export default defineConfig({
+  esbuild: {
+    pure: mode === "production" ? ["console.debug"] : [],
+  },
 });
```

## AC
- [x] `console.debug` calls stripped from production bundle
- [x] `console.error`, `console.warn` preserved
- [x] Dev mode unaffected
