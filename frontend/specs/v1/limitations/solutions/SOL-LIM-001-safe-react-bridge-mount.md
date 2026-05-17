# SOL-LIM-001 — Safe React Bridge Mount With Render Versioning

> **Resolves**: BUG-LIM-001 (Race Condition Trong Vue→React Bridge Mount)  
> **Type**: Code-Level Fix + TDD Update  
> **Priority**: High  
> **Effort**: Small (~2 ngày)  
> **Status**: Proposed

---

## 1. Mục Tiêu

Loại bỏ mọi race condition trong `ReactPageMount.vue → mount.ts` bằng cách:
1. Thay thế serialization queue bằng **render versioning** (generation counter).
2. Capture props snapshot trước khi bắt đầu async work.
3. Validate DOM container connectivity sau mỗi yield point.

---

## 2. Giải Pháp Kỹ Thuật

### 2.1 Render Versioning Pattern (Thay thế renderQueue)

```typescript
// ReactPageMount.vue — NEW implementation
let renderGeneration = 0;

async function render() {
  const gen = ++renderGeneration; // Mỗi render nhận version number duy nhất
  const snapshotPage = props.page;
  const snapshotProps = pageProps.value ? { ...pageProps.value } : undefined;

  const [{ mountReactPage, updateReactPage }, i18nModule] = await Promise.all([
    import("./mount"),
    import("./i18n"),
  ]);

  // Guard 1: Component unmounted hoặc render mới hơn đã khởi tạo
  if (unmounted || gen !== renderGeneration || !container.value) return;

  // Guard 2: DOM container vẫn attached
  if (!container.value.isConnected) return;

  // Sync locale
  if (i18nModule.default.language !== locale.value) {
    await i18nModule.default.changeLanguage(locale.value);
    if (gen !== renderGeneration || unmounted || !container.value?.isConnected) return;
  }

  // Page changed → full remount
  if (root && currentPage !== snapshotPage) {
    root.unmount();
    root = null;
  }

  if (!root) {
    root = await mountReactPage(container.value, snapshotPage, snapshotProps);
    if (gen !== renderGeneration || unmounted) {
      root?.unmount();
      root = null;
      return;
    }
  } else {
    await updateReactPage(root, snapshotPage, snapshotProps);
  }
  currentPage = snapshotPage;
}
```

**Key changes:**
1. `renderGeneration` — Monotonically increasing counter. Mỗi render mới increment. Nếu giữa chừng phát hiện `gen !== renderGeneration` → render cũ tự abort.
2. `snapshotPage` / `snapshotProps` — Props được capture đồng bộ TRƯỚC khi bất kỳ `await` nào chạy → loại bỏ stale props.
3. `container.value.isConnected` — DOM API check xác minh element vẫn trong document tree.

### 2.2 Cleanup Lifecycle Hardening

```typescript
onUnmounted(() => {
  unmounted = true;
  renderGeneration++; // Cancel mọi render đang in-flight
  root?.unmount();
  root = null;
});
```

### 2.3 Race-Free mount.ts (không đổi API)

`mount.ts` không cần thay đổi. Tất cả race protection nằm trong `ReactPageMount.vue`. Tuy nhiên, nên thêm defensive check trong `buildTree()`:

```typescript
function buildTree(deps: ReactDeps, Component: ReactComponent, props?: any) {
  if (!Component) {
    console.error("buildTree called with undefined Component");
    return deps.createElement("div", null, "Component load failed");
  }
  return deps.createElement(deps.StrictMode, null,
    deps.createElement(deps.I18nextProvider, { i18n: deps.i18n },
      deps.createElement(Component, props)
    )
  );
}
```

---

## 3. Thay Đổi Architecture Document

### 3.1 Cập nhật `specs/architecture.md` — Section 4.1 Bridge Pattern

Thêm mô tả render versioning vào phần "Quy trình mount":

> **Mount Safety**: `ReactPageMount.vue` sử dụng render versioning (generation counter) để đảm bảo:
> - Chỉ render mới nhất được apply, các render cũ tự abort sau mỗi yield point.
> - Props được snapshot đồng bộ trước khi bắt đầu async operations.
> - DOM container connectivity được verify qua `isConnected` API.

### 3.2 Cập nhật `specs/technical-design-document.md` — Section 3.1

Thêm vào cuối section "Vue ↔ React Bridge":

> **Race Condition Prevention**: Bridge sử dụng monotonically increasing `renderGeneration` counter. Mỗi `render()` call increment counter và capture snapshot của props tại thời điểm gọi. Sau mỗi `await` point, function check `gen !== renderGeneration` — nếu true, một render mới đã bắt đầu và render hiện tại tự terminate. Pattern này thay thế Promise queue vì nó handle cả trường hợp Vue unmount/remount component.

---

## 4. Test Cases

```typescript
describe("ReactPageMount race conditions", () => {
  test("rapid page switching only mounts final page", async () => {
    // Mount component
    // Quickly change props.page 3 times: A → B → C
    // Assert only C is mounted, A and B are never rendered
  });

  test("unmount during async load does not create root", async () => {
    // Mount component → immediately unmount
    // Assert no React root is created
    // Assert no console errors
  });

  test("props change during mount uses latest props", async () => {
    // Mount with props.open = false
    // During mount await, change props.open = true
    // Assert React component receives open = true
  });
});
```

---

## 5. Acceptance Criteria

| Metric | Current | Target |
|---|---|---|
| Stale props after rapid navigation | Possible | Impossible (snapshot pattern) |
| Double root creation | Possible (rare) | Impossible (generation guard) |
| Blank page on detached container | Possible | Impossible (`isConnected` check) |
| Render queue Promise leak | Possible | N/A (no queue, counter-based) |
