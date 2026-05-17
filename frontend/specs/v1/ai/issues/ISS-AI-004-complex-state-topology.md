# ISS-AI-004 — Topology State Management Đa Tầng Gây Khó Khăn Cho AI

> **Category**: State Management Complexity  
> **Severity**: High  
> **Impact**: State Debugging, Feature Addition, Data Flow Reasoning  
> **Affected Area**: `src/store/`, `src/react/stores/`, `src/store/cache.ts`

---

## 1. Mô Tả Vấn Đề

Codebase triển khai **4 lớp state management đồng thời**, tạo ra topology phức tạp mà AI khó reasoning:

### 1.1 Bốn Lớp State

```
Layer 1: Pinia Stores (Vue)
  ├── 33 domain stores trong src/store/modules/v1/
  ├── 8 SQL Editor stores trong src/store/modules/sqlEditor/
  └── 18 other module stores
  
Layer 2: Zustand Stores (React)
  └── src/react/stores/app/ (aggregated app store)

Layer 3: Request/Entity Cache (Generic)
  └── src/store/cache.ts (dual-layer: request dedup + entity cache)

Layer 4: Vue Reactivity Bridge (Cross-framework)
  └── useVueState() hook — React ←→ Vue reactive subscription
```

### 1.2 Cache Strategy Complexity

`cache.ts` triển khai pattern phức tạp:

```
getRequest(keys) → miss
  → setRequest(keys, promise) 
    → AbortController registration
  → promise.then(result => setEntity(keys, result))
  → invalidateRequest(keys)
    → AbortController.abort() // prevent stale writes
```

- **Namespace-based isolation**: Mỗi store dùng `useCache<[...keys], Entity>("namespace")`.
- **AbortController lifecycle**: AI khó hiểu khi nào request bị abort vs completed.
- **Reactivity leak**: Entity cache dùng `shallowReactive(Map)` — shallow reactivity có thể gây lỗi nếu AI mutate nested fields.

### 1.3 Cross-Store Dependencies

```
Auth Store ← hầu hết stores khác (currentUser check)
Permission Store ← AuthStore, WorkspaceStore, ProjectIamPolicyStore
Subscription Store ← ActuatorStore (server info), SettingStore
Database Store ← InstanceStore, EnvironmentStore, ProjectStore
Issue Store ← PlanStore, RolloutStore, ProjectStore
```

AI thường không thấy implicit dependency chain này.

## 2. Giới Hạn Khi Sử Dụng AI

| Scenario | Giới hạn |
|---|---|
| **Add new data fetch** | AI phải biết: dùng store nào? cache strategy nào? fetch pattern (getOrFetch vs search)? |
| **Debug stale data** | AI cần trace: cache hit/miss → request dedup → AbortController → entity reactivity |
| **Cross-framework state** | AI phải reasoning qua Vue watch → useSyncExternalStore → React re-render cycle |
| **Feature gating** | AI cần biết: SubscriptionStore.hasFeature() → checks PlanFeature enum → license validation |
| **Permission checking** | AI phải trace: route meta → PermissionStore → IAM policy → ProjectIamPolicy |

## 3. Data Flow Khó Trace

```
React Component
  ↓ useVueState(() => useDatabaseV1Store().getOrFetchDatabaseByName(name))
    ↓ Pinia Store
      ↓ getEntity([name]) → cache miss
        ↓ getRequest([name]) → request miss
          ↓ databaseServiceClientConnect.getDatabase({ name })
            ↓ ConnectRPC Transport
              ↓ authInterceptor → activeInterceptor → errorNotificationInterceptor
                ↓ Backend gRPC
              ↓ Response
            ↓ setEntity([name], result)
          ↓ Vue reactive update
        ↓ useSyncExternalStore notification
      ↓ React re-render
```

**8 levels deep** — AI phải understand mỗi layer để debug correctly.

## 4. Khuyến Nghị Giảm Thiểu

1. **Document state flow diagrams per domain**: Tạo visual data flow cho top-10 stores (database, project, issue, instance).
2. **Standardize fetch patterns**: Enforce single pattern cho tất cả stores (hiện tại có 3 variants: `getOrFetch`, `fetchList`, `search`).
3. **Type-safe store exports**: Tạo barrel file cho từng domain với explicit interface, giúp AI biết available methods.
4. **Reduce Pinia ↔ React coupling**: Migrate frequently-used React state sang Zustand store, giảm `useVueState` calls.
