# Change Request: Distributed Cache Layer for HA Mode

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-004                                              |
| **Source ID**      | ARCH-LIM-004                                             |
| **Title**          | Distributed Cache Layer — Break Cache-HA Mutual Exclusion |
| **Category**       | Architecture (Performance + Scaling)                     |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SEC-01 (IAM), DCM-01 (Change Workflow), ADM-08 (API)    |

---

## 1. Tổng quan

### 1.1 Mô tả
Introduce shared cache layer (Redis/Valkey) cho HA mode, thay thế binary choice "cache ON (single-node) XOR cache OFF (HA)". HA mode hiện tại tắt toàn bộ 13 LRU caches, tạo 500x DB load increase cho IAM checks.

### 1.2 Bối cảnh
- HA mode: `enableCache = !profile.HA` → disable tất cả 13 caches
- IAM permission check: 0.01ms (cache hit) vs 5ms (DB query)
- At 10K req/s: 100ms total → 50,000ms total DB time (500x increase)
- 13 caches (capacity: 32K-1K entries) all disabled simultaneously
- No distributed cache infrastructure exists

### 1.3 Mục tiêu
- HA mode retains caching benefit (shared cache)
- IAM check latency < 1ms in HA mode (via Redis)
- DB query volume reduced ≥ 80% compared to current HA (no-cache)
- Fallback: degrade to direct-DB if Redis unavailable

---

## 2. Yêu cầu chức năng

### FR-001: Cache Abstraction Layer
- **Mô tả**: Extract cache interface cho hot-path entities (User, IAM Policy, Setting, Role).
- **Logic**:
  ```go
  // cache/cache.go
  type Cache[K comparable, V any] interface {
      Get(ctx context.Context, key K) (V, bool, error)
      Set(ctx context.Context, key K, value V, ttl time.Duration) error
      Delete(ctx context.Context, key K) error
      Purge(ctx context.Context) error
  }

  // Implementations:
  // - cache.NewLRU[K,V](capacity)       — in-process (single-node)
  // - cache.NewRedis[K,V](client, prefix) — shared (HA mode)
  // - cache.NewNoop[K,V]()               — disabled (testing)
  ```
- **Acceptance Criteria**:
  - AC-1: Store uses `Cache[K,V]` interface thay vì concrete LRU
  - AC-2: Single-node mode: LRU backend (existing behavior)
  - AC-3: HA mode: Redis backend (new behavior)
  - AC-4: Cache miss transparent — falls back to DB query

### FR-002: Redis/Valkey Integration
- **Mô tả**: Connect to Redis/Valkey cluster cho HA cache sharing.
- **Config**:
  ```env
  CACHE_BACKEND=redis          # 'lru' | 'redis' | 'none'
  CACHE_REDIS_URL=redis://redis:6379
  CACHE_REDIS_PREFIX=bb:
  CACHE_DEFAULT_TTL=60s
  ```
- **Acceptance Criteria**:
  - AC-1: All replicas share same cache state
  - AC-2: Cache write-through: update DB → invalidate cache → return
  - AC-3: Serialization: protojson for cache values
  - AC-4: TTL-based expiry (configurable per entity)

### FR-003: Hot-Path Cache Prioritization
- **Mô tả**: Prioritize caching cho high-frequency entities.
- **Priority Map**:

  | Entity | Frequency | Cache TTL | Justification |
  |--------|-----------|-----------|---------------|
  | IAM Policy | Very High | 60s | Every request checks permissions |
  | Roles | Very High | 60s | Role→permission mapping on every ACL check |
  | User | High | 120s | Auth interceptor loads user per request |
  | Setting | Medium | 300s | Server config rarely changes |
  | Project | Medium | 120s | Project lookup on most API calls |
  | Instance | Low | 300s | Instance metadata changes infrequently |

- **Acceptance Criteria**:
  - AC-1: Top 4 caches (IAM, Role, User, Setting) migrated first
  - AC-2: Cache hit rate ≥ 90% for IAM/Role in HA mode

### FR-004: Graceful Cache Degradation
- **Mô tả**: Redis unavailable → fallback to direct DB (not crash).
- **Acceptance Criteria**:
  - AC-1: Redis connection lost → log warning, continue without cache
  - AC-2: Redis reconnected → cache resumes automatically
  - AC-3: Health check reports cache status

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Cache interface        | `backend/store/cache/cache.go`        | Generic Cache[K,V] interface                 |
| LRU adapter            | `backend/store/cache/lru.go`          | Wrap existing hashicorp/golang-lru           |
| Redis adapter          | `backend/store/cache/redis.go`        | go-redis/redis/v9 adapter                    |
| Store integration      | `backend/store/store.go`              | Replace concrete LRU with Cache interface    |
| Config                 | `backend/component/config/profile.go` | Cache backend config parsing                 |

### 3.2 Infrastructure Changes
- **New dependency**: Redis/Valkey instance for HA deployments
- **Helm chart**: Add optional Redis service

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | HA mode with Redis → IAM check latency < 1ms                | Cache hit from Redis                     |
| TC-002     | Two replicas share cache → no stale data                     | Write-through invalidation works         |
| TC-003     | Redis down → system continues (degraded, direct DB)          | No crash, warning logged                 |
| TC-004     | Redis reconnected → cache resumes                            | Auto-recovery                            |
| TC-005     | Single-node mode → LRU backend (no change)                   | Backward compatible                      |
| TC-006     | Cache hit rate ≥ 90% for IAM queries under load              | Load test verification                   |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Cache interface + LRU adapter (refactor, no new behavior) | Sprint 1 |
| Phase 2 | Redis adapter + config integration                 | Sprint 2     |
| Phase 3 | Migrate IAM + Role caches (hot path)               | Sprint 2     |
| Phase 4 | Migrate remaining caches (User, Project, Setting)  | Sprint 3     |
| Phase 5 | Helm chart + Redis deployment docs                 | Sprint 3     |
| Phase 6 | Load testing: HA mode with Redis vs without        | Sprint 4     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Redis adds infrastructure complexity           | MEDIUM | Optional — LRU still works for single-node          |
| Cache serialization overhead                   | LOW    | protojson fast enough for small entities             |
| Network latency Redis → replicas               | LOW    | Co-locate Redis with app pods                        |
| Cache stampede on Redis restart               | MEDIUM | Staggered TTL + singleflight for concurrent loads   |
