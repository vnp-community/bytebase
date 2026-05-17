# TASK-W-030: Lazy Monaco Loading

> **Source**: SOL-WEAK-006 §2.5 | **Priority**: P3 | **Effort**: 1.5h  
> **Status**: DONE | **Deps**: —

## Scope
- **EDIT** `src/views/sql-editor/EditorPanel/index.vue`

## What
Replace static `import { editor } from "monaco-editor"` with `defineAsyncComponent` to defer 3MB+ chunk until editor tab is actually opened.

## Implementation — see SOL-WEAK-006 §2.5 diff
```diff
-import { editor } from "monaco-editor";
+const MonacoEditor = defineAsyncComponent(() =>
+  import("@/components/MonacoEditor/MonacoEditor.vue")
+);
```

## AC
- [x] Monaco chunk not loaded until editor tab opened
- [x] SQL Editor still works correctly when tab opened
- [x] Initial page load faster (3MB+ deferred)
