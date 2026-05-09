# Change Request: Deep Health Check with Readiness/Liveness

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-008                                              |
| **Source ID**      | ARCH-WEAK-003                                            |
| **Title**          | Deep Health Check — Component-Level Health Reporting     |
| **Category**       | Architecture (Operational Visibility)                    |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | ADM-08 (API Integration), SEC-10 (Audit Log)            |

---

## 1. Tổng quan

### 1.1 Mô tả
Thay thế shallow `/healthz` (luôn trả 200 OK) bằng deep health check system với component-level reporting. Hỗ trợ Kubernetes liveness/readiness probes.

### 1.2 Bối cảnh
- `/healthz` hiện tại: `return c.String(http.StatusOK, "OK!")` — luôn 200
- Không verify: DB connectivity, runner health, license validity, disk space
- Kubernetes/load balancer không thể phát hiện instance unhealthy
- Broken instances tiếp tục nhận traffic

### 1.3 Mục tiêu
- `/healthz` → deep health check (DB ping, runner status)
- `/readyz` → readiness probe (schema migrated, runners started)
- `/livez` → liveness probe (process alive, DB reachable)
- Component-level health reporting JSON response
- K8s probe compatible response codes

---

## 2. Yêu cầu chức năng

### FR-001: Health Check Endpoint Redesign
- **Mô tả**: `/healthz` returns component-level health status.
- **Logic**:
  ```json
  // GET /healthz → 200 (all healthy) or 503 (degraded)
  {
    "status": "healthy|degraded|unhealthy",
    "components": {
      "database": { "status": "healthy", "latency_ms": 2 },
      "runners": { "status": "healthy", "active": 8, "expected": 8 },
      "cache": { "status": "healthy", "hit_rate": 0.95 },
      "license": { "status": "valid", "expires": "2027-01-01" },
      "pool": { "status": "healthy", "api_active": 12, "api_max": 35 }
    },
    "version": "v2.15.0",
    "uptime_seconds": 86400
  }
  ```
- **Acceptance Criteria**:
  - AC-1: DB ping failure → status "unhealthy", HTTP 503
  - AC-2: Runner goroutine died → status "degraded", HTTP 503
  - AC-3: All healthy → HTTP 200
  - AC-4: Response latency < 100ms (timeout on slow checks)

### FR-002: Readiness Probe
- **Mô tả**: `/readyz` — is server ready to receive traffic?
- **Checks**:
  - Database connected and responsive
  - Schema migration completed
  - All critical runners started
  - IAM cache populated
- **Acceptance Criteria**:
  - AC-1: Returns 200 only after full init completed
  - AC-2: Returns 503 during startup/migration

### FR-003: Liveness Probe
- **Mô tả**: `/livez` — lightweight, is process functioning?
- **Checks**:
  - DB ping (with 1s timeout)
  - Process not deadlocked
- **Acceptance Criteria**:
  - AC-1: < 10ms response time
  - AC-2: Minimal checks — no cache or runner verification

### FR-004: Health Check Caching
- **Mô tả**: Cache health results for 5 seconds to prevent check storms.
- **Acceptance Criteria**:
  - AC-1: Concurrent health requests share cached result
  - AC-2: Cache TTL configurable

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Health handler         | `backend/server/health.go`            | New: deep health check implementation        |
| Echo routes            | `backend/server/echo_routes.go`       | Add /readyz, /livez, update /healthz         |
| Component registry     | `backend/server/components.go`        | Track component health status                |
| Runner health          | `backend/runner/*/`                   | Report goroutine alive status                |
| Pool health            | `backend/store/pool_metrics.go`       | Connection pool utilization                  |

### 3.2 Kubernetes Integration

```yaml
# Recommended K8s probe configuration
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 5
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | DB disconnected → /healthz returns 503                       | Unhealthy reported                       |
| TC-002     | All healthy → /healthz returns 200 + JSON                    | Full component status                    |
| TC-003     | During startup → /readyz returns 503                         | Not ready during init                    |
| TC-004     | After startup → /readyz returns 200                          | Ready after init complete                |
| TC-005     | /livez responds in < 10ms                                    | Lightweight check                        |
| TC-006     | Runner goroutine panics → /healthz shows degraded            | Runner health tracked                    |
| TC-007     | Concurrent health requests → cached response                 | No check storm                           |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Implement /livez (simple DB ping)                  | Sprint 1     |
| Phase 2 | Implement /readyz (startup state tracking)         | Sprint 1     |
| Phase 3 | Implement /healthz (component-level reporting)     | Sprint 2     |
| Phase 4 | Runner health goroutine tracking                   | Sprint 2     |
| Phase 5 | Helm chart update + K8s probe documentation        | Sprint 3     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Health check itself becomes slow              | MEDIUM | 1s timeout on all checks, 5s result caching         |
| DB ping adds load                             | LOW    | Max 1 ping per 5 seconds (cached)                   |
| Backward compatibility (tools expect "OK!")   | LOW    | /healthz still returns 200 when healthy              |
