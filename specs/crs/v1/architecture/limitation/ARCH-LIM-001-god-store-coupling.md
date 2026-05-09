# ARCH-LIM-001 — God Object Store: Central Coupling Point

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Limitation (Structural Trade-off)          |
| **Layer**      | L8 (Data Access — Store)                   |
| **Impact**     | Testability, Modularity, Cognitive Load    |
| **Severity**   | High                                       |

---

## 1. Description

`*store.Store` là central dependency cho toàn bộ hệ thống. **73 files** trong backend depend on `*store.Store` trực tiếp (verified via `grep -l '\*store\.Store'`).

Store struct chứa **74 data access files** + 13 LRU caches, cung cấp 200+ public methods. Tất cả layers L3-L7 đều inject `*store.Store` qua constructor — tạo thành **God Object** pattern.

### Evidence

```go
// grpc_routes.go — mọi service nhận *store.Store
aiService := apiv1.NewAIService(stores)
authService := apiv1.NewAuthService(stores, secret, licenseService, profile, iamManager)
databaseService := apiv1.NewDatabaseService(stores, schemaSyncer, profile, iamManager, licenseService)
// ... 30+ services tất cả nhận stores
```

### Metrics
- **73** files depend on `*store.Store` (concrete type)
- **200+** public methods on Store struct
- **0** interface definitions trong `backend/store/` (no `interfaces.go`)
- **13** LRU caches bundled vào Store struct

---

## 2. Root Cause Analysis

### Design Decision
Bytebase chọn single Store struct thay vì repository-per-entity pattern để:
1. Simplify transaction management (single `*sql.DB`)
2. Centralize cache invalidation logic
3. Avoid import cycles giữa entity-specific store packages

### Why It Became a Problem
- Store grew organically — từ simple CRUD thành 200+ methods
- No interface extraction → impossible to mock → untestable services
- Adding new entity requires modifying the God Object
- Cognitive load: developer phải understand toàn bộ Store API

---

## 3. Dependency Diagram

```
                    ┌────────────────────────────┐
                    │    *store.Store (L8)        │
                    │  74 files, 200+ methods     │
                    │  13 LRU caches              │
                    └──────────┬─────────────────┘
                               │
     ┌────────┬────────┬───────┼───────┬────────┬───────┐
     ▼        ▼        ▼       ▼       ▼        ▼       ▼
   L3(ACL)  L4(30+    L5     L6      L7(some) L9     L10
            Services) (IAM,   (Task,    (IDP)  (Lic.) (LSP,
                      Bus,    Schema,                  MCP)
                      Webhook) Approval)
```

### Counter-Pattern Comparison

Well-designed repository pattern:
```
AuthService → UserRepository (interface, 5 methods)
             → TokenRepository (interface, 3 methods)

PlanService → PlanRepository (interface, 8 methods)
             → TaskRepository (interface, 6 methods)
```

Current Bytebase:
```
AuthService → *store.Store (200+ methods accessible)
PlanService → *store.Store (same 200+ methods accessible)
```

---

## 4. Consequences

| Consequence | Description |
|------------|-------------|
| **Untestable** | Cannot mock Store → tests require real DB (testcontainers) → slow CI |
| **Implicit Dependencies** | Service uses 5 methods from Store but has access to 200+ → unclear contract |
| **Cache Coupling** | Cache invalidation logic embedded in Store → cannot swap cache strategy |
| **No Compile-Time Safety** | Adding/removing Store methods doesn't generate compile errors in consumers |
| **Cognitive Overload** | New developer must understand entire Store API to work on single service |

---

## 5. Measurement

| Metric | Current | Target |
|--------|---------|--------|
| Files depending on `*store.Store` | 73 | < 20 (via interfaces) |
| Store public methods | 200+ | — (refactored behind interfaces) |
| Interface definitions in store/ | 0 | 12+ (per-entity) |
| Unit tests without DB | 0 | 60%+ of service tests |
