# BUG-WEAK-002: Entity Cache Unbounded Growth & Stale Data

> **Severity**: MEDIUM-HIGH  
> **Category**: State Management Bug  
> **Affected Files**: `src/store/cache.ts`, `src/store/modules/v1/database.ts`, `src/store/modules/v1/project.ts`, `src/store/modules/v1/instance.ts`  
> **Status**: OPEN  
> **Created**: 2026-05-13

---

## 1. Mô tả

Hệ thống cache dual-layer (Request Cache + Entity Cache) không có cơ chế TTL, eviction, hoặc size limit → memory tăng không kiểm soát trong các phiên làm việc dài.

---

## 2. Chi tiết lỗi

### 2.1 Entity Cache Không Có Eviction Policy

**File**: `src/store/cache.ts` (L14-21, L84-91)

```typescript
const REQUEST_CACHE = new Map<string, Map<string, RequestCacheEntry<...>>>();
const ENTITY_CACHE = new Map<string, Map<string, EntityCacheEntry<...>>>();
```

**Vấn đề**:
- `ENTITY_CACHE` là **global singleton** Maps, chỉ có `clear()` method → **không có TTL, LRU eviction, hoặc max size**.
- Với 33 domain stores sử dụng cache, user duyệt nhiều projects/databases → hàng nghìn entities tích lũy.
- `clear()` chỉ được gọi trong `reset()` → chỉ xảy ra khi logout, **không clear giữa các workspace switch**.

### 2.2 Database Store Request Cache Không Bao Giờ Invalidated

**File**: `src/store/modules/v1/database.ts` (L157, L289-293)

```typescript
const databaseRequestCache = new Map<string, Promise<Database>>();

const getOrFetchDatabaseByName = async (name: string, silent = true) => {
  // ...
  const cached = databaseRequestCache.get(name);
  if (cached) return cached;
  const request = fetchDatabaseByName(name, silent);
  databaseRequestCache.set(name, request);
  return request; // <-- Promise cached forever, even if it rejects
};
```

**Vấn đề**:
- `databaseRequestCache` là **separate** từ store's dual-layer cache — tạo **triple cache layer**.
- Promise cached **forever** — nếu fetch ban đầu thành công nhưng database bị rename/delete/transfer sau đó → stale data persist.
- Nếu Promise reject (network error), rejected promise vẫn cached → **subsequent calls sẽ nhận rejected promise**.

### 2.3 Dual Cache Key Strategy Inconsistency

**File**: `src/store/cache.ts` (L151-153)

```typescript
const getKey = (keys: KeyType[]) => {
  return JSON.stringify(keys);
};
```

**Vấn đề**:
- `JSON.stringify` cho cache keys có potential collision nếu key contains special characters.
- `database.ts` dùng `databaseRequestCache` (Map riêng) với string key trực tiếp, trong khi hàm `useCache` dùng `JSON.stringify([name])` → inconsistent key format giữa 2 cache layers.

### 2.4 Console Debug Traces Active in Production

**File**: `src/store/cache.ts` (L27-29)

```typescript
const trace = (title: string, keys: KeyType[], ...args: unknown[]) => {
  console.debug("cache", namespace, title, JSON.stringify(keys), ...args);
};
```

**Vấn đề**:
- `console.debug` gọi trên mỗi `getRequest`, `getEntity` call — trên production với `console.debug` enabled, mỗi API interaction sinh ra log spam.
- Serialization (`JSON.stringify`) trên hot path = unnecessary CPU overhead.

---

## 3. Tác động

| Impact | Mô tả |
|--------|--------|
| **Memory Bloat** | Entity cache tăng liên tục trong phiên dài (> 1 giờ), đặc biệt với workspace có nhiều databases |
| **Stale Data** | Entities đã bị xóa/đổi tên vẫn tồn tại trong cache → UI hiển thị thông tin outdated |
| **Failed Request Cache** | Rejected promises cached → calls tiếp theo fail ngay lập tức mà không retry |
| **Performance** | Console.debug + JSON.stringify trên hot path gây jank không cần thiết |

---

## 4. Đề xuất Fix

1. **TTL-based eviction** — thêm `createdAt` vào EntityCacheEntry, sweep entries > 5 phút
2. **Max size policy** — LRU eviction khi cache vượt 500 entries per namespace
3. **Remove rejected promise caching** — clear `databaseRequestCache` entry khi Promise reject
4. **Consolidate cache layers** — loại bỏ `databaseRequestCache`, dùng chung `useCache` pattern
5. **Guard console.debug** — wrap behind `isDev()` check hoặc remove
