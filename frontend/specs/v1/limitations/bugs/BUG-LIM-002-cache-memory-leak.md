# BUG-LIM-002 — Entity Cache Tăng Trưởng Không Giới Hạn (Memory Leak)

> **Category**: Memory Management  
> **Severity**: High  
> **Impact**: Performance Degradation, Browser Crash (Long Sessions)  
> **Affected Files**: `src/store/cache.ts`, `src/store/modules/v1/*.ts` (33 stores)

---

## 1. Mô Tả Vấn Đề

`cache.ts` triển khai dual-layer cache (Request Cache + Entity Cache) sử dụng `Map` không có cơ chế eviction (TTL, LRU, max-size). Tất cả 33 domain stores sử dụng cache pattern này, khiến bộ nhớ tăng liên tục theo thời gian sử dụng.

### 1.1 Không Có TTL/Max-Size

```typescript
// cache.ts:84-91
const setEntity = (keys: K, entity: T) => {
  const key = getKey(keys);
  entityCacheMap.set(key, { keys, entity });
  // Không có check max-size, TTL, hoặc eviction policy
};
```

Mỗi lần fetch entity mới, nó được thêm vào Map vĩnh viễn. Không có cơ chế tự động xóa entries cũ.

### 1.2 Clear Chỉ Khi Logout

```typescript
// cache.ts:99-109
const clear = () => {
  for (const request of requestCacheMap.values()) {
    if (!request.abortController.signal.aborted) {
      request.abortController.abort();
    }
  }
  requestCacheMap.clear();
  entityCacheMap.clear();
};
```

`clear()` chỉ được gọi khi user logout hoặc navigate ra khỏi module. Trong session dài (admin duyệt hàng trăm databases, issues, plans), cache sẽ tích lũy hàng ngàn entities.

### 1.3 Reactive Map Chi Phí Cao

```typescript
// cache.ts:146
const created = shallowReactive(new Map<string, EntityCacheEntry<K, T>>());
```

`shallowReactive(Map)` khiến Vue tracking mọi change, tạo overhead cho garbage collector khi Map lớn.

## 2. Tác Động Cụ Thể

| Store | Entity Size (ước tính) | Tích lũy sau 1h sử dụng |
|---|---|---|
| `database` | ~5-10KB/entity | 200+ databases = ~2MB |
| `dbSchema` | ~50-200KB/entity | 50+ schemas = ~10MB |
| `issue` | ~3-8KB/entity | 300+ issues = ~2.4MB |
| `project` | ~2-5KB/entity | 50+ projects = ~250KB |
| `worksheet` | ~1-3KB/entity | 100+ worksheets = ~300KB |
| **Total** | | **~15-20MB+ trong RAM** |

Đặc biệt nguy hiểm với `dbSchema` store — mỗi database schema có thể chứa hàng trăm tables/columns, tạo entity rất lớn.

## 3. Reproduction Steps

1. Login vào Bytebase với workspace có 100+ databases.
2. Duyệt qua từng database → view schema → view changelogs.
3. Mở Chrome DevTools → Memory tab → Take Heap Snapshot.
4. Lặp lại bước 2-3 sau 30 phút.
5. **Quan sát**: Heap size tăng đều mà không giảm khi navigate khỏi các pages.

## 4. Root Cause

- `useCache()` được thiết kế cho **session-scoped caching** nhưng không có upper bound.
- 33 stores sử dụng cùng pattern, mỗi store tạo namespace riêng nhưng tất cả đều global (`MODULE_LEVEL Map` — `REQUEST_CACHE`, `ENTITY_CACHE`).
- Không có eviction khi entity bị stale (ví dụ: database bị xóa, issue bị đóng).

## 5. Khuyến Nghị

1. **Thêm LRU eviction**: Sử dụng LRU Map (hoặc `lru-cache` package) với max-size per namespace.
2. **TTL cho entity cache**: Entities nên có timestamp, tự invalidate sau N phút.
3. **Selective clear on navigation**: Khi user rời module (ví dụ: từ databases sang settings), clear cache của module cũ.
4. **WeakRef cho large entities**: Sử dụng `WeakRef` cho entities lớn (`dbSchema`), cho phép GC thu hồi khi cần.
5. **Monitor cache size**: Thêm dev-mode logging cho cache size để phát hiện leak sớm.
