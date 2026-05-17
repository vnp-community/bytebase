# BUG-WEAK-006: Performance & Bundle Weaknesses

> **Severity**: MEDIUM  
> **Category**: Performance Bug  
> **Status**: OPEN | **Created**: 2026-05-13

## 1. Mô tả

Nhiều vấn đề performance ảnh hưởng đến load time, runtime responsiveness, và memory footprint.

## 2. Chi tiết lỗi

### 2.1 Blocking Bootstrap — Sequential Awaits Before Mount
**File**: `src/main.ts` (L49-62)

```typescript
const currentUser = await useAuthStore().fetchCurrentUser();  // Blocking!
const initPromises = [useActuatorV1Store().fetchServerInfo(...)];
if (currentUser) {
  initPromises.push(useSubscriptionV1Store().fetchSubscription());
  initPromises.push(useWorkspaceV1Store().fetchWorkspaceList());
}
await Promise.all(initPromises);  // Blocking!
app.mount("#app");  // App chỉ mount SAU KHI tất cả API calls hoàn thành
```

- User nhìn thấy blank page cho đến khi **3-4 API calls** hoàn thành.
- Trên mạng chậm (3G) → 3-5 giây blank page trước khi thấy bất kỳ UI.
- `AuthContext.vue` fetch thêm 5 API calls nữa → tổng 8-9 sequential calls trước khi interactive.

### 2.2 Node Memory Limit 8GB cho Build
**File**: `package.json` / Build config

- Build yêu cầu `--max_old_space_size=8000` (8GB).
- Nguyên nhân: `types/proto-es/` generated code rất lớn + dual framework compilation.
- CI/CD pipelines cần machine >= 16GB RAM → tốn chi phí.

### 2.3 Query Parameter Preservation — Deep Comparison on Every Navigation
**File**: `src/App.vue` (L163-181)

```typescript
watch(route, (current, prev) => {
  const fields = ["mode", "customTheme", "lang", "project", "filter"];
  const preservedQuery = cloneDeep(current.query);
  // ...
  if (isEqual(current.query, preservedQuery)) { return; }
  router.replace({ ...current, query: preservedQuery });
});
```

- `cloneDeep` + `isEqual` (lodash) chạy trên **mỗi route change**.
- `router.replace()` trigger thêm 1 navigation → route watcher chạy lại → potential loop (guarded bởi `isEqual`).

### 2.4 Dual i18n Bundle — Translation Files Duplicated
- Vue: `src/locales/` — 5 languages × ~130KB = ~650KB
- React: `src/react/locales/` — same 5 languages với overlapping translations.
- Total i18n payload ~1.2MB+ raw, chưa tính SQL Review locale files.
- Nhiều translation keys trùng lặp giữa 2 hệ thống.

### 2.5 Monaco Editor Chunk Dominates Bundle
- Monaco chunk là largest bundle — 3MB+ compressed.
- Loaded lazily nhưng `SQLEditorLayout` preloads nó → impact initial load cho SQL Editor users.

### 2.6 Console Debug Statements in Production
**File**: `src/router/index.ts` L86, `src/main.ts` L29-30, `src/store/cache.ts` L28

```typescript
console.debug("Router %s -> %s", from.name, to.name);  // Every navigation
console.debug("dev:", isDev());   // On boot
console.debug("cache", ...);     // Every cache operation
```

- Debug logs không bị strip trong production build (chỉ gated bởi browser console level).

## 3. Đề xuất Fix
1. **Progressive rendering** — mount app immediately với skeleton, fetch data in background
2. **Merge i18n** — shared namespace giữa Vue/React, single source file
3. **Tree-shake console.debug** — dùng build-time replacement hoặc conditional
4. **Optimize route watcher** — dùng shallow comparison thay vì `cloneDeep` + `isEqual`
5. **Lazy Monaco** — chỉ load khi user thực sự navigate đến SQL Editor
