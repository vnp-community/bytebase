# BUG-WEAK-001: Vue ↔ React Bridge Race Conditions & Memory Leaks

> **Severity**: HIGH  
> **Category**: Architectural Bug  
> **Affected Files**: `src/react/mount.ts`, `src/react/ReactPageMount.vue`, `src/react/mountSidebar.ts`, `src/react/mountProjectSidebar.ts`  
> **Status**: OPEN  
> **Created**: 2026-05-13

---

## 1. Mô tả

Cơ chế bridge Vue ↔ React sử dụng dynamic `import.meta.glob()` + `createRoot()` có nhiều race condition tiềm ẩn và rủi ro memory leak khi chuyển đổi route nhanh.

---

## 2. Chi tiết lỗi

### 2.1 React Root Leak khi Fast Navigation

**File**: `src/react/ReactPageMount.vue` (L82-96)

```typescript
if (root && currentPage !== props.page) {
  root.unmount();
  root = null;
}
if (!root) {
  root = await mountReactPage(container.value, props.page, pageProps.value);
  // ...
}
```

**Vấn đề**: 
- `mountReactPage()` là `async` — giữa lúc await và lúc gán `root`, nếu user navigate sang route khác, `onUnmounted` callback sẽ chạy với `root = null` (chưa được gán) → React root vừa tạo sẽ **không bao giờ bị unmount**, gây memory leak.
- Mặc dù có guard `if (unmounted)` sau await, nhưng React root đã được tạo và render trước khi kiểm tra → orphaned React root tồn tại trong memory.

### 2.2 Cached Page Components Never Invalidated

**File**: `src/react/mount.ts` (L38, L86-98)

```typescript
const cachedPages = new Map<string, ReactComponent>();

async function loadPage(name: string): Promise<ReactComponent> {
  const hit = cachedPages.get(name);
  if (hit) return hit;
  // ...
  cachedPages.set(name, Component);
  return Component;
}
```

**Vấn đề**:
- `cachedPages` là module-level Map, **không bao giờ bị clear**, kể cả khi user logout.
- Nếu component chứa closure references đến stale state (ví dụ: Pinia store snapshot lúc mount), thông tin nhạy cảm có thể persist qua sessions.

### 2.3 Type Safety Bypass trong Bridge Layer

**File**: `src/react/mount.ts` (L32-35)

```typescript
type ReactDeps = any;
type ReactComponent = (props: any) => any;
```

**Vấn đề**:
- Toàn bộ bridge layer bỏ qua type checking → props sai kiểu sẽ không bị phát hiện lúc compile time.
- Cùng pattern `any` lặp lại ở `mountSidebar.ts` và `mountProjectSidebar.ts`.
- Risk: silent runtime failures khi React component nhận props sai.

### 2.4 Render Queue Error Swallowing

**File**: `src/react/ReactPageMount.vue` (L56-63)

```typescript
function render(): Promise<void> {
  const next = renderQueue.then(() => doRender());
  renderQueue = next.catch(() => undefined); // <-- Error silently swallowed
  return next;
}
```

**Vấn đề**: 
- Mọi lỗi từ `doRender()` bị swallow bởi `.catch(() => undefined)` trên chain.
- Nếu dynamic import fail (network error, missing module), user thấy blank page **không có error notification**.

---

## 3. Tác động

| Impact | Mô tả |
|--------|--------|
| **Memory Leak** | React roots không được unmount khi navigate nhanh, tích lũy qua thời gian → browser tab chậm dần |
| **Stale Data** | Cached components có thể giữ reference đến stale auth/user state |
| **Silent Failures** | Render errors bị swallow → blank pages không diagnostic |
| **Type Safety** | Props mismatch chỉ phát hiện runtime, không compile time |

---

## 4. Reproduction Steps

1. Mở SQL Editor page
2. Navigate nhanh liên tục giữa Settings → Project → Settings (click rất nhanh)
3. Mở DevTools → Memory → Take heap snapshot
4. Tìm kiếm "ReactDOMRoot" — có thể thấy nhiều orphaned instances
5. Lặp lại bước 2-4 nhiều lần → heap size tăng dần

---

## 5. Đề xuất Fix

1. **Sử dụng AbortController** cho mount operations — cancel pending mount khi unmount hoặc page change
2. **Clear cachedPages on logout** — thêm cleanup vào auth store reset flow  
3. **Thêm error boundary** cho render queue — hiển thị error notification thay vì swallow
4. **Định nghĩa typed interfaces** cho bridge props — thay thế `any` bằng generic constraints
