# Change Request: Multi-Tenant Workspace Isolation & Resource Governance

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-004                                              |
| **Title**          | Multi-Tenant Workspace Isolation for 100+ Banks          |
| **Category**       | Performance / Multi-Tenancy                              |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SEC-01, SEC-14, ADM-01, ADM-05                           |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế tenant isolation layer cho >100 bank tenants trên shared Bytebase deployment. Hiện tại `workspace` chỉ là filter column — không có resource quota, rate limiting, hay noisy neighbor protection. Một bank có thể consume toàn bộ connection pool, cache, hoặc sync capacity.

### 1.2 Bối cảnh
- `workspace` field tồn tại trên `instance` table nhưng không trên `db` table (phải JOIN)
- Không có per-tenant resource quota (databases, instances, connections)
- Không có per-tenant rate limiting cho API calls
- Schema syncer không phân biệt tenant priority
- Cache không isolate giữa tenants — hot tenant evict cold tenant
- IAM check per-request nhưng không batch/cache per-tenant

### 1.3 Mục tiêu
- Hoàn toàn isolate data giữa 100+ bank tenants
- Resource quota enforcement per tenant
- Rate limiting per tenant cho API protection
- Noisy neighbor protection: 1 tenant không ảnh hưởng tenant khác
- Tenant-level monitoring và alerting

---

## 2. Yêu cầu chức năng

### FR-001: Tenant Resource Quotas
- **Mô tả**: Configurable resource limits per tenant
- **Quotas**:
  | Resource          | Default    | Configurable |
  |-------------------|------------|--------------|
  | Max Instances     | 100        | ✅           |
  | Max Databases     | 5,000      | ✅           |
  | Max Projects      | 50         | ✅           |
  | Max Users         | 200        | ✅           |
  | Max Concurrent Queries | 50    | ✅           |
  | Max DB Size (metadata) | 10GB  | ✅           |
- **AC**:
  - AC-1: Create instance/database fails with `RESOURCE_EXHAUSTED` khi exceed quota
  - AC-2: Dashboard hiển thị quota usage per tenant
  - AC-3: Quota configurable via API + Setting UI

### FR-002: Per-Tenant API Rate Limiting
- **Mô tả**: Token bucket rate limiter per tenant per API endpoint group
- **Rate limits**:
  | Endpoint Group    | Default Rate | Burst |
  |-------------------|-------------|-------|
  | Read (List/Get)   | 1000 req/s  | 2000  |
  | Write (Create/Update) | 100 req/s | 200  |
  | Schema Sync       | 10 req/s    | 50    |
  | SQL Query         | 200 req/s   | 500   |
- **AC**:
  - AC-1: Rate limit exceeded returns `429 Too Many Requests`
  - AC-2: Rate limit headers in response (X-RateLimit-*)
  - AC-3: Rate limits configurable per tenant

### FR-003: Connection Pool Isolation
- **Mô tả**: Dedicated connection sub-pools per tenant group
- **Logic**: Pool partitioned proportionally to tenant database count
- **AC**:
  - AC-1: Tenant with 50K DBs cannot exhaust pool of tenant with 1K DBs
  - AC-2: Min 5 connections per tenant guaranteed
  - AC-3: Overflow pool (20% of total) shared for burst

### FR-004: Tenant-Level Monitoring Dashboard
- **Mô tả**: Prometheus metrics và Grafana dashboards per tenant
- **Metrics**:
  - `bytebase_tenant_databases_total{workspace="bank-a"}`
  - `bytebase_tenant_api_requests_total{workspace, method, status}`
  - `bytebase_tenant_cache_hit_ratio{workspace}`
  - `bytebase_tenant_sync_duration_seconds{workspace}`
  - `bytebase_tenant_quota_usage_ratio{workspace, resource}`
- **AC**:
  - AC-1: All metrics labeled with workspace
  - AC-2: Alert khi quota usage >80%
  - AC-3: Alert khi tenant API error rate >5%

### FR-005: Tenant Onboarding Automation
- **Mô tả**: Automated workflow cho onboarding new bank tenant
- **Steps**: Create workspace → Set quotas → Create admin user → Configure environments → Activate license
- **AC**:
  - AC-1: New tenant onboarded via single API call
  - AC-2: Default quota template applied
  - AC-3: Audit log captures onboarding events

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component          | File                                     | Thay đổi                              |
|--------------------|------------------------------------------|----------------------------------------|
| Quota Manager      | `backend/component/quota/manager.go`     | New: tenant resource quota enforcement |
| Rate Limiter       | `backend/component/ratelimit/limiter.go` | New: per-tenant token bucket           |
| API Interceptor    | `backend/api/v1/interceptor.go`          | Add rate limit + quota check           |
| Pool Partitioner   | `backend/store/pool_partition.go`        | New: tenant-aware connection pools     |
| Metrics            | `backend/metrics/tenant_metrics.go`      | New: per-tenant Prometheus metrics     |
| Onboarding API     | `backend/api/v1/tenant_service.go`       | New: tenant lifecycle management       |
| Store — Quota      | `backend/store/quota.go`                 | New: quota storage and retrieval       |

### 3.2 Database Changes

```sql
CREATE TABLE tenant_quota (
    workspace TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    max_value BIGINT NOT NULL,
    current_value BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace, resource_type)
);

CREATE TABLE tenant_rate_limit (
    workspace TEXT NOT NULL,
    endpoint_group TEXT NOT NULL,
    rate_per_second INT NOT NULL,
    burst_size INT NOT NULL,
    PRIMARY KEY (workspace, endpoint_group)
);
```

### 3.3 Configuration

| Environment Variable          | Default | Mô tả                                |
|-------------------------------|---------|----------------------------------------|
| `TENANT_QUOTA_ENABLED`        | `true`  | Enable quota enforcement               |
| `TENANT_RATE_LIMIT_ENABLED`   | `true`  | Enable per-tenant rate limiting        |
| `TENANT_POOL_ISOLATION`       | `true`  | Enable connection pool partitioning    |
| `TENANT_OVERFLOW_POOL_RATIO`  | `0.2`   | Shared overflow pool ratio             |
| `TENANT_MIN_CONNECTIONS`      | `5`     | Min connections guaranteed per tenant  |

---

## 4. Performance Targets

| Metric                        | Current         | Target (100+ tenants) |
|-------------------------------|-----------------|----------------------|
| Tenant isolation              | None            | Full data isolation  |
| Cross-tenant impact           | Unbounded       | ≤ 5% degradation    |
| Quota check latency           | N/A             | ≤ 0.5ms (cached)    |
| Rate limit check latency      | N/A             | ≤ 0.1ms             |
| Tenant onboarding time        | Manual (~1hr)   | ≤ 30s automated     |

---

## 5. Test Cases

| Test ID | Mô tả                                         | Expected Result                |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | Tenant exceeds database quota                  | Create blocked, 429 returned  |
| TC-002  | Tenant API rate limit exceeded                 | 429 with Retry-After header   |
| TC-003  | Heavy tenant cannot exhaust shared pool        | Other tenants unaffected      |
| TC-004  | 100 tenants concurrent API calls               | P99 ≤ 100ms per tenant        |
| TC-005  | New tenant onboarding via API                  | Complete within 30s           |
| TC-006  | Quota increase via admin API                   | Immediate effect              |
| TC-007  | Tenant data isolation: list returns own only   | Zero cross-tenant leakage    |
| TC-008  | Prometheus metrics per workspace label         | All metrics partitioned       |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline   |
|---------|--------------------------------------|------------|
| Phase 1 | Quota tables + enforcement layer    | Sprint 1-2 |
| Phase 2 | Per-tenant rate limiting            | Sprint 2   |
| Phase 3 | Connection pool partitioning        | Sprint 3   |
| Phase 4 | Tenant monitoring + alerting        | Sprint 3   |
| Phase 5 | Onboarding automation               | Sprint 4   |
| Phase 6 | 100-tenant load testing             | Sprint 4   |

---

## 7. Risks & Mitigations

| Risk                           | Impact | Mitigation                              |
|--------------------------------|--------|-----------------------------------------|
| Quota check adds latency       | LOW    | Cache quotas in-memory, async refresh   |
| Rate limiter memory per tenant | LOW    | Token bucket is O(1) per tenant         |
| Pool fragmentation             | MEDIUM | Overflow pool + dynamic rebalancing     |
| Quota bypass via batch APIs    | HIGH   | Enforce quota on batch endpoints too    |
| Tenant count exceeds partition | LOW    | Hash-based partitioning scales linearly |
