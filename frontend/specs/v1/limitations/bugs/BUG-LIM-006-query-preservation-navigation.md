# BUG-LIM-006 — Query Preservation Loop và Navigation Side Effects

> **Category**: Routing / Navigation  
> **Severity**: Medium  
> **Impact**: Infinite Redirect Loop, Unexpected URL Mutation  
> **Affected Files**: `src/App.vue` (lines 162-181), `src/router/index.ts`

---

## 1. Mô Tả Vấn Đề

### 1.1 Query Preservation Gây Side Effects

```typescript
// App.vue:162-181
watch(route, (current, prev) => {
  const fields = ["mode", "customTheme", "lang", "project", "filter"];
  const preservedQuery = cloneDeep(current.query);
  for (const key of fields) {
    if (preservedQuery[key] === undefined) {
      preservedQuery[key] = prev.query[key];
    }
  }
  if (isEqual(current.query, preservedQuery)) {
    return;
  }
  router.replace({
    ...current,
    query: preservedQuery,
  });
});
```

**Vấn đề 1 — Infinite Loop tiềm ẩn**: `router.replace()` trigger route change → watcher fire lại → check `isEqual` → nếu Vue Router normalize query khác với `cloneDeep` → loop.

**Vấn đề 2 — Stale query preservation**: Nếu user navigate từ page A (có `filter=active`) sang page B (không cần filter), filter vẫn bị carry over → page B có thể hiểu sai query params.

**Vấn đề 3 — Interaction với React Router**: Trong standalone React mode, React Router quản lý URL riêng. Query preservation ở Vue level có thể conflict.

### 1.2 Open Redirect Partial Mitigation

```typescript
// router/index.ts:156-163
if (relayState && typeof relayState === "string") {
  if (relayState.startsWith("/") && !relayState.startsWith("//")) {
    redirect = relayState;
  }
}
```

**Vấn đề**: Chỉ kiểm tra `//` prefix nhưng không check:
- `javascript:` URLs (unlikely nhưng possible qua encoding)
- Path traversal patterns
- Encoded characters (`%2F%2F` = `//`)
- `redirectParam` KHÔNG được validate — có thể chứa absolute URL:
  ```typescript
  } else if (redirectParam) {
    redirect = redirectParam;  // ← Không validate!
  }
  ```

### 1.3 Store Reset Race Condition

```typescript
// router/index.ts:170-178
if (isAuthRelatedRoute(to.name as string)) {
  useDatabaseV1Store().reset();
  useProjectV1Store().reset();
  useInstanceV1Store().reset();
  import("@/plugins/ai/store").then(({ useConversationStore }) => {
    useConversationStore().reset();
  });
  next();  // ← next() gọi TRƯỚC khi AI store reset xong
  return;
}
```

**Vấn đề**: `import()` là async nhưng `next()` được gọi ngay lập tức. Navigation đã hoàn thành trước khi AI conversation store được reset → stale AI state trên auth pages.

## 2. Tác Động

| Scenario | Xác suất | Hậu quả |
|---|---|---|
| Query preservation loop | Low | Browser tab freeze do infinite redirects |
| Stale filter carry-over | Medium | Sai kết quả tìm kiếm/filtering trên page mới |
| Open redirect via `redirect` param | Low (but security) | Phishing attack qua crafted login URL |
| AI store stale on auth pages | Low | Sensitive conversation data visible on signin page |

## 3. Root Cause

- Query preservation watch không đủ defensive checks.
- Open redirect validation không complete (chỉ cover `relay_state`, không cover `redirect`).
- Async store reset không await trước khi proceed navigation.

## 4. Khuyến Nghị

1. **Debounce query preservation**: Thêm `{ flush: 'post' }` cho watch và guard bằng navigation state flag.
2. **Validate ALL redirect params**: Apply cùng validation cho `redirect` như `relay_state`:
   ```typescript
   const isValidRedirect = (url: string) => 
     url.startsWith("/") && !url.startsWith("//") && !url.includes(":");
   ```
3. **Await store resets**: 
   ```typescript
   await import("@/plugins/ai/store").then(m => m.useConversationStore().reset());
   next();
   ```
4. **Explicit query whitelist per route**: Thay vì global preservation, mỗi route meta nên declare accepted query params.
