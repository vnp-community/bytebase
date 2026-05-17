# SOL-LIM-005 — Browser Baseline Upgrade & Polyfill Cleanup

> **Resolves**: BUG-LIM-005 (WeakRef Polyfill Gây Memory Leak Nghịch Lý)  
> **Type**: Configuration Change  
> **Priority**: Medium  
> **Effort**: Small (~1 ngày)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Loại bỏ WeakRef polyfill gây memory leak bằng cách nâng browser baseline và dọn dẹp polyfill layer.

---

## 2. Giải Pháp

### 2.1 Nâng Browser Baseline

**Thay đổi `package.json` `browserslist`:**

```diff
  "browserslist": [
-   "> 0.08%, not dead"
+   "Chrome >= 84, Firefox >= 79, Safari >= 14.1, Edge >= 84, not dead"
  ]
```

**Rationale**: WeakRef được support từ Chrome 84 (2020-07), Firefox 79 (2020-07), Safari 14.1 (2021-04). Tất cả browser versions này đã >4 năm tuổi — coverage >98% global users.

### 2.2 Xóa WeakRef Polyfill

**Xóa file**: `src/polyfill.ts` (hoặc remove WeakRef block, giữ lại polyfills khác nếu có).

**Nếu cần giữ polyfill cho API compatibility** (graceful degradation), simplify:

```typescript
// src/polyfill.ts — SIMPLIFIED
(() => {
  if (typeof WeakRef === "undefined") {
    console.warn(
      "[Bytebase] Your browser does not support WeakRef. " +
      "Memory usage may increase over time. Please upgrade your browser."
    );
    // Simplified no-op polyfill: target is never GC'd (documented behavior)
    (globalThis as any).WeakRef = class WeakRefShim<T extends WeakKey> {
      readonly [Symbol.toStringTag] = "WeakRef";
      private readonly _target: T;
      constructor(target: T) { this._target = target; }
      deref(): T | undefined { return this._target; }
    };
  }
})();
```

### 2.3 Review Legacy Plugin Targets

**Cập nhật `vite.config.ts`:**

```diff
  legacy({
-   targets: ["> 0.08%, not dead"],
+   targets: ["Chrome >= 84, Firefox >= 79, Safari >= 14.1, Edge >= 84"],
    additionalLegacyPolyfills: ["regenerator-runtime/runtime"],
  }),
```

---

## 3. Thay Đổi Architecture Document

### 3.1 Cập nhật `specs/architecture.md` — Section 13.2 Known Constraints

Xóa hoặc update WeakRef constraint. Thêm:

> - **Browser minimum**: Chrome 84+, Firefox 79+, Safari 14.1+, Edge 84+ (WeakRef, Web Locks, BroadcastChannel required)

---

## 4. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| WeakRef polyfill memory leak | Active on old browsers | Eliminated (user warned) |
| Browser compatibility warnings | None | Console warning on unsupported browser |
| Polyfill code complexity | Map + double strong ref | Single field, no Map |
| Browser support coverage | >99.9% (with leak) | >98% (without leak) |
