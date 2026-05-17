# BUG-LIM-005 — WeakRef Polyfill Gây Memory Leak Nghịch Lý

> **Category**: Browser Compatibility / Memory  
> **Severity**: Medium  
> **Impact**: Memory Leak trên Browser Cũ  
> **Affected Files**: `src/polyfill.ts`

---

## 1. Mô Tả Vấn Đề

```typescript
// polyfill.ts:3-21
class WeakRefPolyfill<T extends WeakKey> {
  private targetMap = new Map<WeakRefPolyfill<T>, T | undefined>();
  private target: T;

  constructor(target: T) {
    this.target = target;
    this.targetMap.set(this, target);
  }

  deref(): T | undefined {
    const target = this.targetMap.get(this);
    return target ?? undefined;
  }
}
```

### 1.1 Polyfill Giữ Strong Reference

`WeakRefPolyfill` sử dụng `Map` (STRONG reference) thay vì thực sự giữ weak reference. Điều này có nghĩa:
- `this.target` giữ strong reference → target object KHÔNG BAO GIỜ bị GC.
- `this.targetMap` (Map) cũng giữ strong reference → double retention.
- Mục đích của `WeakRef` (cho phép GC thu hồi target) **hoàn toàn bị đảo ngược**.

### 1.2 Instance-Level Map Lãng Phí

```typescript
private targetMap = new Map<WeakRefPolyfill<T>, T | undefined>();
```

Mỗi `WeakRefPolyfill` instance tạo MỘT Map riêng, trong đó chỉ lưu DUY NHẤT một entry (chính nó). Đây là overhead vô nghĩa — một field `private target` đã đủ.

### 1.3 Phạm Vi Ảnh Hưởng

Polyfill chỉ active khi `typeof WeakRef === "undefined"`, tức browser rất cũ. Tuy nhiên:
- `browserslist: "> 0.08%, not dead"` target browser rất rộng.
- `@vitejs/plugin-legacy` tạo polyfill chunks cho browser cũ.
- Nếu user dùng browser cũ → tất cả `WeakRef` usage trong codebase (Vue reactivity internals, entity cache) sẽ bị memory leak.

## 2. Tác Động

| Scenario | Xác suất | Hậu quả |
|---|---|---|
| Browser cũ (IE 11, Safari < 14.1) | Low | Tất cả WeakRef objects retain targets forever |
| Vue 3 internal WeakRef usage | N/A | Vue 3 yêu cầu WeakRef native, polyfill chỉ cho app code |
| Long-running session trên browser cũ | Low | Gradual memory growth |

## 3. Root Cause

- `WeakRef` polyfill bản chất là **impossible** trong JavaScript thuần vì language không expose GC primitives.
- Polyfill đúng cách phải document rằng nó KHÔNG thực sự weak — chỉ cung cấp API compatibility.
- Field `this.target` thừa (đã có trong `targetMap`), tạo double strong reference.

## 4. Khuyến Nghị

1. **Remove polyfill hoặc warn**: Nếu target browsers yêu cầu WeakRef, polyfill này sẽ gây hại hơn có lợi. Nên:
   - Raise minimum browser requirement (`WeakRef` supported từ Chrome 84, Firefox 79, Safari 14.1).
   - Hoặc log warning khi polyfill active: `console.warn("WeakRef polyfill active — memory usage may increase")`.

2. **Simplify nếu giữ**: Remove `targetMap`, chỉ giữ `this.target`:
   ```typescript
   class WeakRefPolyfill<T extends WeakKey> {
     private target: T;
     constructor(target: T) { this.target = target; }
     deref(): T | undefined { return this.target; }
   }
   ```

3. **Update browserslist**: Nâng minimum target browser lên version hỗ trợ WeakRef native.
