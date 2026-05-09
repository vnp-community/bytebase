# Change Request: Connection Pool Isolation (API vs Runner)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-007                                              |
| **Source ID**      | ARCH-WEAK-002                                            |
| **Title**          | Connection Pool Isolation — Dual Pool Architecture       |
| **Category**       | Architecture (Performance + Reliability)                 |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01 (Change Workflow), DCM-09 (Batch Changes), SQL-01 (SQL Editor) |

---

## 1. Tổng quan

### 1.1 Mô tả
Tách single connection pool (50 conn hard cap) thành **dual pools**: API pool (user-facing) và Runner pool (background). Schema sync, migration, data export runners hiện tại compete với API requests cho cùng connections.

### 1.2 Bối cảnh
- 1 `*sql.DB` pool cho tất cả: 8 runners + 30+ API services
- `maxOpenConns` hard cap tại 50 (db_connection.go:242-246)
- Schema sync có thể scan hàng trăm instances đồng thời
- API requests queue when runners exhaust pool → user-visible latency

### 1.3 Mục tiêu
- API requests guaranteed 70% pool capacity (35/50 conns)
- Runner workloads capped at 30% pool (15/50 conns)
- API p99 latency unaffected by background operations
- Prometheus metrics per pool (utilization, wait time, errors)

---

## 2. Yêu cầu chức năng

### FR-001: PoolManager with Dual Pools
- **Mô tả**: Replace single `DBConnectionManager` with `PoolManager` managing 2 pools.
- **Logic**:
  ```go
  type PoolManager struct {
      apiPool    *sql.DB   // 70% connections — latency-sensitive
      runnerPool *sql.DB   // 30% connections — throughput-oriented
  }
  func (p *PoolManager) APIPool() *sql.DB    { return p.apiPool }
  func (p *PoolManager) RunnerPool() *sql.DB { return p.runnerPool }
  ```
- **Config**:
  ```env
  PG_MAX_CONNECTIONS=50
  PG_API_POOL_RATIO=0.7      # 35 conns for API
  PG_RUNNER_POOL_RATIO=0.3   # 15 conns for runners
  ```
- **Acceptance Criteria**:
  - AC-1: API services use `APIPool()` exclusively
  - AC-2: Runners use `RunnerPool()` exclusively
  - AC-3: Total connections = API + Runner ≤ PG_MAX_CONNECTIONS
  - AC-4: Pool ratio configurable via environment variables

### FR-002: Store Pool Routing
- **Mô tả**: Store methods receive pool context to route queries.
- **Logic**:
  ```go
  // Option 1: Context-based routing
  ctx = store.WithPool(ctx, store.PoolAPI)
  user, err := s.GetUser(ctx, ...)

  // Option 2: Separate Store instances
  apiStore := store.NewWithPool(apiPool)
  runnerStore := store.NewWithPool(runnerPool)
  ```
- **Acceptance Criteria**:
  - AC-1: Zero change in query logic — only pool selection changes
  - AC-2: Default pool = API pool (safe default)

### FR-003: Pool Metrics
- **Mô tả**: Prometheus metrics per pool.
- **Metrics**:
  - `bytebase_db_pool_active_conns{pool="api"|"runner"}` — active connections
  - `bytebase_db_pool_idle_conns{pool="api"|"runner"}` — idle connections
  - `bytebase_db_pool_wait_duration_seconds{pool}` — time waiting for conn
  - `bytebase_db_pool_max_conns{pool}` — configured max

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| PoolManager            | `backend/store/pool_manager.go`       | New: dual pool initialization + management   |
| Store integration      | `backend/store/store.go`              | Accept PoolManager, route queries by pool    |
| Runner injection       | `backend/runner/taskrun/scheduler.go` | Use RunnerPool                               |
| Runner injection       | `backend/runner/schemasync/syncer.go` | Use RunnerPool                               |
| API services           | `backend/api/v1/*.go`                 | Use APIPool (default, minimal changes)       |
| Metrics                | `backend/store/pool_metrics.go`       | Prometheus collectors per pool               |
| Config                 | `backend/component/config/profile.go` | Pool ratio configuration                     |

### 3.2 Database/Frontend Changes
Không có.

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Schema sync 100 instances → API latency unchanged            | API p99 < 50ms during sync               |
| TC-002     | Runner pool exhausted → API pool unaffected                  | API queries continue normally            |
| TC-003     | API pool exhausted → runners still process                   | Runners continue (separate pool)         |
| TC-004     | Total connections ≤ PG_MAX_CONNECTIONS                       | No PG connection leak                    |
| TC-005     | Prometheus metrics show per-pool utilization                 | Grafana dashboard functional             |
| TC-006     | Pool ratio 0.7/0.3 → 35 API + 15 runner conns              | Config applied correctly                 |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | PoolManager implementation + unit tests            | Sprint 1     |
| Phase 2 | Store integration + runner pool injection          | Sprint 1-2   |
| Phase 3 | Prometheus metrics + Grafana dashboard             | Sprint 2     |
| Phase 4 | Load testing: compare API latency before/after     | Sprint 3     |
| Phase 5 | Production deploy with configurable ratios         | Sprint 3     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Total conn count exceeds PG limits            | HIGH   | sum(pools) ≤ pg max_connections - 5 (reserved)      |
| Runner pool too small for heavy syncs         | MEDIUM | Configurable ratio, monitor wait times               |
| Connection leak in one pool                   | MEDIUM | Per-pool idle timeout + connection health check      |
| Transaction spanning both pools               | LOW    | Transactions use single pool, never cross-pool       |
