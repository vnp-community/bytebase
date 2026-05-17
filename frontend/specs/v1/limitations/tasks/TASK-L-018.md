# TASK-L-018: Simplify WeakRef Polyfill

> **Source**: SOL-LIM-005 §2.2 | **Priority**: P3 | **Effort**: 0.5h  
> **Status**: DONE | **Deps**: L-017

## Scope
- **EDIT** `src/polyfill.ts`

## What
Thay thế WeakRef polyfill phức tạp (Map + double strong ref) bằng simplified shim chỉ giữ 1 strong ref + console warning.

## Implementation

```diff
-// Complex polyfill with Map-based tracking (causes memory leak)
-if (typeof WeakRef === "undefined") {
-  const refs = new Map<number, object>();
-  let nextId = 0;
-  (globalThis as any).WeakRef = class WeakRefShim<T extends WeakKey> {
-    private id: number;
-    constructor(target: T) { this.id = nextId++; refs.set(this.id, target); }
-    deref(): T | undefined { return refs.get(this.id) as T | undefined; }
-  };
-}
+if (typeof WeakRef === "undefined") {
+  console.warn(
+    "[Bytebase] Your browser does not support WeakRef. " +
+    "Memory usage may increase over time. Please upgrade your browser."
+  );
+  (globalThis as any).WeakRef = class WeakRefShim<T extends WeakKey> {
+    readonly [Symbol.toStringTag] = "WeakRef";
+    private readonly _target: T;
+    constructor(target: T) { this._target = target; }
+    deref(): T | undefined { return this._target; }
+  };
+}
```

## AC
- [ ] No `Map` used in polyfill (eliminates double-reference leak)
- [ ] Console warning shown on unsupported browsers
- [ ] `deref()` still returns target (graceful degradation)
