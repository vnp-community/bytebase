# Change Request: Distributed Cache & HA Horizontal Scaling

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-LIM-001                                               |
| **Limitation ID**  | LIM-001                                                  |
| **Title**          | Distributed Cache Layer & HA Horizontal Scaling          |
| **Category**       | Architecture / Scalability                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Giải quyết giới hạn horizontal scaling của kiến trúc monolith bằng cách đưa vào **distributed cache layer** (Redis), **leader election** cho background runners, và hỗ trợ **read replicas** cho metadata PostgreSQL.

### 1.2 Bối cảnh
Hiện tại HA mode buộc phải tắt toàn bộ LRU cache (`enableCache=false`), gây tăng latency 3-5x và giảm concurrent capacity 50%. Background runners chạy duplicate trên tất cả replicas mà không có coordination. Đây là rào cản lớn nhất cho production deployment quy mô lớn.

### 1.3 Mục tiêu
- Khôi phục hiệu suất cache trong HA mode qua distributed cache (Redis/Valkey)
- Loại bỏ duplicate work của runners qua leader election
- Giảm load lên primary PostgreSQL qua read replica routing
- Tăng concurrent capacity từ ~50-100 lên ~500+ users trong HA mode

---

## 2. Yêu cầu chức năng

### FR-001: Distributed Cache Layer (Redis)
- **Mô tả**: Thay thế in-process LRU cache bằng Redis/Valkey cache layer cho HA mode.
- **Logic**:
  ```
  IF profile.HA:
      cache = NewRedisCache(redisURL, ttl_config)
  ELSE:
      cache = NewLRUCache(32768)  // giữ nguyên single-node behavior
  ```
- **Cache Entities**: User, Instance, Database, Project, Policy, Setting
- **TTL Strategy**:
  | Entity     | TTL       | Invalidation             |
  |------------|-----------|--------------------------|
  | User       | 5 min     | On update + PG NOTIFY    |
  | Instance   | 10 min    | On update + PG NOTIFY    |
  | Database   | 10 min    | On update + PG NOTIFY    |
  | Project    | 15 min    | On update + PG NOTIFY    |
  | Policy     | 5 min     | On update + PG NOTIFY    |
  | Setting    | 30 min    | On update + PG NOTIFY    |
- **Acceptance Criteria**:
  - AC-1: HA mode với Redis cache đạt latency ≤ 120% so với single-node LRU cache
  - AC-2: Cache invalidation propagate tới tất cả replicas trong < 1 giây
  - AC-3: Redis connection failure fallback về direct DB query (graceful degradation)
  - AC-4: Cache hit ratio ≥ 85% trong steady-state operation

### FR-002: Leader Election cho Background Runners
- **Mô tả**: Implement leader election để chỉ **một replica** chạy background runners tại mỗi thời điểm.
- **Mechanism**: PostgreSQL Advisory Locks (tận dụng infrastructure hiện có, không thêm dependency)
- **Logic**:
  ```
  FOR each runner IN [taskScheduler, schemaSyncer, approvalRunner, planCheckScheduler, ...]:
      IF acquireAdvisoryLock(runner.lockID):
          runner.Run(ctx)  // only leader runs
      ELSE:
          runner.Standby(ctx)  // standby monitors lock
  ```
- **Acceptance Criteria**:
  - AC-1: Chỉ 1 replica chạy mỗi runner tại bất kỳ thời điểm nào
  - AC-2: Failover time < 30 giây khi leader crash
  - AC-3: Không có duplicate work (schema sync, approval check, etc.)
  - AC-4: Health endpoint report runner leadership status

### FR-003: Read Replica Routing cho Metadata Queries
- **Mô tả**: Hỗ trợ routing read-only queries tới PostgreSQL read replicas.
- **Logic**:
  ```
  IF query.isReadOnly AND readReplicaURL != "":
      execute on readReplicaPool
  ELSE:
      execute on primaryPool
  ```
- **Applicable Queries**: List operations, search, dashboard aggregation, audit log reads
- **Acceptance Criteria**:
  - AC-1: Cấu hình `PG_READ_REPLICA_URL` environment variable
  - AC-2: Read-only queries route tới replica, write queries luôn đi primary
  - AC-3: Replica lag tolerance configurable (default: 5s, skip replica nếu lag > threshold)
  - AC-4: Giảm ≥ 40% load lên primary PostgreSQL

### FR-004: Connection Pool Scaling
- **Mô tả**: Tăng connection pool capacity và làm configurable.
- **Logic**:
  - Default pool size: `max(50, numCPU * 10)`
  - Configurable via `PG_MAX_CONNECTIONS` environment variable
  - Separate pools cho primary (read-write) và replica (read-only)
- **Acceptance Criteria**:
  - AC-1: Pool size configurable qua environment variable
  - AC-2: Pool metrics (active, idle, waiting) exported tới Prometheus
  - AC-3: Connection pool exhaustion trả về clear error message

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                             | Thay đổi                                          |
|------------------------------|------------------------------------------|----------------------------------------------------|
| Cache Interface              | `backend/store/cache.go`                 | Abstract cache interface (`Cache[K,V]`)            |
| LRU Cache Adapter            | `backend/store/cache_lru.go`             | Wrap existing LRU as `Cache[K,V]` implementation   |
| Redis Cache Adapter          | `backend/store/cache_redis.go`           | Redis-backed `Cache[K,V]` implementation           |
| Cache Invalidator            | `backend/store/cache_invalidator.go`     | PG NOTIFY → Redis invalidation bridge              |
| Store Constructor            | `backend/store/store.go`                 | Accept cache type config, init Redis if HA         |
| Leader Election              | `backend/component/leader/election.go`   | Advisory lock-based leader election                |
| Runner Wrapper               | `backend/runner/leader_runner.go`        | Wrap runners with leader election guard            |
| Server Initialization        | `backend/server/server.go`               | Wire leader election into runner startup           |
| DB Pool Manager              | `backend/store/pool.go`                  | Primary + Replica connection pool management       |
| Health Check                 | `backend/api/v1/actuator_service.go`     | Report cache, leader, pool status                  |
| Prometheus Metrics           | `backend/metrics/cache_metrics.go`       | Cache hit/miss, pool utilization, leader status    |

### 3.2 Configuration

| Environment Variable       | Default          | Mô tả                                    |
|----------------------------|------------------|-------------------------------------------|
| `REDIS_URL`                | _(empty)_        | Redis connection URL cho distributed cache |
| `REDIS_PASSWORD`           | _(empty)_        | Redis authentication                      |
| `REDIS_TLS_ENABLED`        | `false`          | Enable TLS cho Redis connection           |
| `PG_READ_REPLICA_URL`     | _(empty)_        | PostgreSQL read replica connection URL     |
| `PG_MAX_CONNECTIONS`       | `50`             | Max connection pool size                   |
| `CACHE_TTL_DEFAULT`        | `600`            | Default cache TTL in seconds               |
| `LEADER_ELECTION_TTL`     | `30`             | Leader lock renewal interval in seconds    |

### 3.3 Database Changes

```sql
-- Thêm notify triggers cho cache invalidation
CREATE OR REPLACE FUNCTION notify_cache_invalidation()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('cache_invalidation',
        json_build_object(
            'table', TG_TABLE_NAME,
            'action', TG_OP,
            'id', COALESCE(NEW.id, OLD.id)
        )::text
    );
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Apply triggers cho cached entities
CREATE TRIGGER trg_user_cache_invalidation
    AFTER INSERT OR UPDATE OR DELETE ON principal
    FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();

-- Tương tự cho: instance, db, project, policy, setting
```

### 3.4 Frontend Changes
Không yêu cầu thay đổi frontend — tất cả là backend infrastructure.

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| Redis/Valkey 7+         | External dependency cho distributed cache (chỉ HA mode)        |
| PostgreSQL 14+          | Tận dụng Advisory Locks + LISTEN/NOTIFY hiện có                |
| `github.com/redis/go-redis/v9` | Go Redis client library                                 |
| PG Read Replica         | Optional — managed PG service với read replica support          |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                        |
|------------|-----------------------------------------------------------------|----------------------------------------|
| TC-001     | HA mode khởi động với REDIS_URL configured                     | Redis cache initialized, LRU disabled  |
| TC-002     | HA mode khởi động không có REDIS_URL                           | Fallback to no-cache (current behavior)|
| TC-003     | Cache invalidation: update user → cache miss trên replica khác | Cache entry evicted within 1s          |
| TC-004     | Redis connection lost during operation                          | Graceful fallback to DB, error logged  |
| TC-005     | Leader election: start 3 replicas                               | Only 1 runs each runner                |
| TC-006     | Leader crash → standby takeover                                | New leader within 30s                  |
| TC-007     | Read query routing to replica                                   | Query executed on replica pool         |
| TC-008     | Write query with read replica configured                        | Query executed on primary pool         |
| TC-009     | Replica lag exceeds threshold                                   | Fallback to primary for reads          |
| TC-010     | Connection pool exhaustion                                      | Clear error, no silent hang            |
| TC-011     | Cache hit ratio under steady load (1000 req/s)                  | ≥ 85% hit ratio                        |
| TC-012     | Prometheus metrics endpoint                                     | Cache, pool, leader metrics present    |

---

## 6. Performance Targets

| Metric                      | Current (HA, no cache) | Target (HA, Redis cache) |
|-----------------------------|------------------------|--------------------------|
| P50 query latency           | ~15ms                  | ≤ 5ms                   |
| P99 query latency           | ~80ms                  | ≤ 20ms                  |
| Max concurrent users        | ~50-100                | ~500+                    |
| DB load (queries/sec)       | 100%                   | ≤ 40%                   |
| Runner duplicate work       | N replicas × work      | 1× work                 |

---

## 7. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Cache interface abstraction + Redis adapter      | Sprint 1-2     |
| Phase 2 | PG NOTIFY cache invalidation triggers            | Sprint 2       |
| Phase 3 | Leader election cho runners                      | Sprint 3       |
| Phase 4 | Read replica routing                             | Sprint 4       |
| Phase 5 | Connection pool improvements + metrics           | Sprint 4       |
| Phase 6 | Load testing + performance validation            | Sprint 5       |
| Phase 7 | Documentation + Helm chart updates               | Sprint 5       |

---

## 8. Risks & Mitigations

| Risk                                    | Impact | Mitigation                                           |
|-----------------------------------------|--------|------------------------------------------------------|
| Redis dependency adds operational cost  | MEDIUM | Optional — single-node vẫn dùng LRU                 |
| Cache consistency issues                | HIGH   | PG NOTIFY + short TTL + graceful degradation         |
| Advisory lock stuck after crash         | MEDIUM | TTL-based lock renewal, automatic cleanup            |
| Read replica lag causes stale reads     | LOW    | Configurable lag threshold, fallback to primary      |
