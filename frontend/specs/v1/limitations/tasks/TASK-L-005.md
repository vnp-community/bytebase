# TASK-L-005: Integrate ErrorBoundary in mount.ts

> **Source**: SOL-LIM-004 §2.2 | **Priority**: P1 | **Effort**: 1h  
> **Status**: DONE | **Deps**: L-004

## Scope
- **EDIT** `src/react/mount.ts` (buildTree function + mountReactPage)

## What
Wrap React component tree với `ErrorBoundary` trong `buildTree()`. Truyền `pageName` từ `mountReactPage()`.

## Implementation

```diff
+import { ErrorBoundary } from "./components/ErrorBoundary";

-function buildTree(deps: ReactDeps, Component: ReactComponent, props?: any) {
+function buildTree(deps: ReactDeps, Component: ReactComponent, props?: any, pageName?: string) {
+  if (!Component) {
+    console.error("buildTree called with undefined Component");
+    return deps.createElement("div", null, "Component load failed");
+  }
   return deps.createElement(
-    deps.StrictMode,
-    null,
+    ErrorBoundary,
+    { pageName },
     deps.createElement(
-      deps.I18nextProvider,
-      { i18n: deps.i18n },
-      deps.createElement(Component, props)
+      deps.StrictMode,
+      null,
+      deps.createElement(
+        deps.I18nextProvider,
+        { i18n: deps.i18n },
+        deps.createElement(Component, props)
+      )
     )
   );
 }

 export async function mountReactPage(container, page, props) {
   const [deps, Component] = await Promise.all([loadCoreDeps(), loadPage(page)]);
   const root = deps.createRoot(container);
-  root.render(buildTree(deps, Component, props));
+  root.render(buildTree(deps, Component, props, page));
   return root;
 }
```

## AC
- [ ] `ErrorBoundary` is outermost wrapper in React tree
- [ ] `pageName` appears in error notifications
- [ ] Undefined component → "Component load failed" fallback (not crash)
- [ ] No change to existing mount API signatures
