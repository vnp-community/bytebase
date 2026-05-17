# TASK-L-012: Defensive buildTree in mount.ts

> **Source**: SOL-LIM-001 §2.3 | **Priority**: P2 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/react/mount.ts` (buildTree function)

## What
Thêm null check cho Component parameter trong `buildTree()`. Trả về error div thay vì crash.

## Implementation

```diff
 function buildTree(deps: ReactDeps, Component: ReactComponent, props?: any) {
+  if (!Component) {
+    console.error("buildTree called with undefined Component");
+    return deps.createElement("div", null, "Component load failed");
+  }
   return deps.createElement(deps.StrictMode, null,
     deps.createElement(deps.I18nextProvider, { i18n: deps.i18n },
       deps.createElement(Component, props)
     )
   );
 }
```

## AC
- [ ] `buildTree(deps, undefined)` renders fallback div (not crash)
- [ ] Console error logged when Component is undefined
- [ ] Normal flow unchanged when Component is valid
