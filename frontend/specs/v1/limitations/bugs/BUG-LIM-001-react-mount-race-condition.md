# BUG-LIM-001 — Race Condition Trong Vue→React Bridge Mount

> **Category**: Runtime Bug  
> **Severity**: High  
> **Impact**: UI Rendering, State Corruption  
> **Affected Files**: `src/react/ReactPageMount.vue`, `src/react/mount.ts`

---

## 1. Mô Tả Vấn Đề

`ReactPageMount.vue` thực hiện mount React component bất đồng bộ (async) thông qua chuỗi `render() → doRender() → loadCoreDeps() + loadPage() → createRoot().render()`. Mặc dù đã có serialization queue (`renderQueue`) để giảm thiểu race, vẫn tồn tại các edge case gây lỗi:

### 1.1 Stale Props Sau Remount

```typescript
// ReactPageMount.vue:82-96
if (root && currentPage !== props.page) {
  root.unmount();
  root = null;
}
if (!root) {
  root = await mountReactPage(container.value, props.page, pageProps.value);
  // ← Giữa unmount() và await mountReactPage(), 
  //    pageProps có thể đã thay đổi do Vue reactivity
}
```

**Triệu chứng**: Khi user navigate nhanh giữa 2 React pages (ví dụ: `MembersPage → SubscriptionPage`), props được capture tại thời điểm bắt đầu mount nhưng React component nhận được có thể nhận props cũ vì `pageProps.value` đã thay đổi trong quá trình await.

### 1.2 Unmount Sau Khi Container Bị Detach

```typescript
// ReactPageMount.vue:71-78
if (unmounted || !container.value) return;
// Sau dòng này, trong thời gian await import(), 
// container.value vẫn tồn tại nhưng DOM element đã bị detach khỏi document
```

**Triệu chứng**: React root được tạo trên DOM element đã bị detach, gây silent rendering failure. Lỗi này xuất hiện khi user Close All tabs trong SQL Editor hoặc switch tab nhanh.

### 1.3 Double Root Creation

Mặc dù `renderQueue` giải quyết phần lớn race condition, đoạn comment tại dòng 46-52 thừa nhận lỗi này đã từng xảy ra:

> "Without serialization a watcher firing during the initial mount can race the onMounted call. Both pass the `if (!root)` guard, both call `mountReactPage`, and React ends up with two roots on the same container"

Serialization queue chỉ đảm bảo sequential execution trong cùng một component instance, không bảo vệ khỏi trường hợp Vue unmount/remount component.

## 2. Tác Động

| Scenario | Xác suất | Hậu quả |
|---|---|---|
| Navigate nhanh giữa React pages | Medium | Props mismatch, hiển thị data sai page |
| Close All tabs + navigate | Low | React silent render failure, blank page |
| Rapid toggle Vue↔React states | Low | Double React root, memory leak + console error |
| Tab switch trong SQL Editor | Medium | Stale worksheet/connection state |

## 3. Root Cause

- Bridge pattern `Vue → React` sử dụng **imperative mount/unmount** thay vì declarative rendering.
- React roots được quản lý thủ công bằng `let root: any = null` (mutable variable), không thuộc Vue reactivity system.
- `import.meta.glob()` lazy loading tạo ra multiple yield points giữa guard checks.

## 4. Khuyến Nghị

1. **Implement AbortController pattern**: Mỗi `doRender()` nên tạo AbortController mới, cancel lần render cũ khi có render mới.
2. **Capture props snapshot**: Capture `pageProps.value` thành local const trước khi bắt đầu async operations.
3. **Container validity check**: Verify `container.value.isConnected` (DOM API) sau mỗi await point.
4. **React 19 concurrent features**: Xem xét sử dụng `startTransition` + `useId` để quản lý mount lifecycle.
