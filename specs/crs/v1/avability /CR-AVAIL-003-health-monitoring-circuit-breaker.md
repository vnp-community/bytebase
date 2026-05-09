# Change Request: Health Monitoring, Circuit Breaker & Self-Healing

| Field              | Value                                                       |
|--------------------|-------------------------------------------------------------|
| **CR ID**          | CR-AVAIL-003                                                |
| **Title**          | Health Monitoring, Circuit Breaker & Self-Healing            |
| **Category**       | Availability / Observability                                |
| **Priority**       | P0 — Critical                                               |
| **Status**         | Draft                                                       |
| **Created**        | 2026-05-08                                                  |
| **Author**         | VNP AI Ops Team                                             |
| **Regulatory**     | FFIEC BCM, PCI-DSS 4.0 Req 10.7, SBV TT09/2020 Điều 12    |

---

## 1. Tổng quan

### 1.1 Mô tả
Implement hệ thống **health monitoring** toàn diện với **circuit breaker pattern** cho tất cả external dependencies, kết hợp **self-healing mechanisms** để tự động phục hồi từ transient failures — đáp ứng yêu cầu giám sát liên tục của ngành tài chính.

### 1.2 Bối cảnh
Hệ thống hiện tại có các gaps về monitoring và resilience:
- **Health check** chỉ có `/healthz` đơn giản — không kiểm tra dependencies
- **Không có circuit breaker** — cascading failures khi database/external service down
- **Không có self-healing** — transient errors yêu cầu manual restart
- **Metrics giới hạn** — thiếu SLI/SLO tracking theo chuẩn tài chính
- **Alerting cơ bản** — không đủ tiered alerting cho on-call rotation

### 1.3 Mục tiêu
- **Deep health check** cho tất cả internal/external dependencies
- **Circuit breaker** pattern cho database, Redis, external APIs
- **Self-healing** cho transient failures (auto-reconnect, auto-restart)
- **SLI/SLO monitoring** theo chuẩn Google SRE + financial compliance
- **Tiered alerting** với escalation procedures

### 1.4 Tiêu chuẩn áp dụng

| Standard                          | Requirement                                          |
|-----------------------------------|------------------------------------------------------|
| PCI-DSS 4.0 — Req 10.7           | Timely detection of failures                         |
| SBV TT09/2020 — Điều 12          | Giám sát hệ thống CNTT liên tục                     |
| FFIEC IT Handbook                 | Monitoring, alerting, incident response              |
| SRE Best Practices                | SLI/SLO framework, error budgets                     |

---

## 2. Yêu cầu chức năng

### FR-001: Deep Health Check System
- **Mô tả**: Multi-layer health check system kiểm tra tất cả dependencies.
- **Health Check Components**:
  ```
  DeepHealthChecker:
      checks = [
          PostgreSQLCheck:
              - Connection alive (ping)
              - Query execution (SELECT 1)
              - Replication lag (if replica configured)
              - Connection pool utilization
          RedisCheck (if HA mode):
              - Connection alive (PING)
              - Memory usage < threshold
              - Key count within limits
          RunnerCheck:
              - Leader election status
              - Runner goroutine alive
              - Task processing rate > 0
          DiskCheck:
              - Data directory space > 10%
              - Temp directory writable
          MemoryCheck:
              - Heap usage < 85% of limit
              - GC pause time < 100ms P99
          ExternalServicesCheck:
              - License server reachable (if cloud)
              - IDP endpoints reachable (if SSO configured)
      ]

  GET /healthz:
      RETURN { status: "ok" } IF process alive  // Liveness

  GET /readyz:
      RETURN aggregate(checks.critical)  // Readiness

  GET /healthz/deep:
      RETURN {
          status: aggregate(all_checks),
          checks: [
              { name, status, latency, message, lastChecked }
          ],
          uptime: server.uptime,
          version: build.version
      }
  ```
- **Acceptance Criteria**:
  - AC-1: `/healthz` responds < 10ms (liveness only)
  - AC-2: `/readyz` responds < 500ms with dependency checks
  - AC-3: `/healthz/deep` provides granular component status
  - AC-4: Failed dependency check transitions readiness to false

### FR-002: Circuit Breaker Pattern
- **Mô tả**: Circuit breaker cho mỗi external dependency ngăn cascading failure.
- **State Machine**:
  ```
  CircuitBreaker States:
      CLOSED   → Normal operation, counting failures
      OPEN     → All requests fail-fast, skip dependency
      HALF-OPEN → Allow limited requests to test recovery

  State Transitions:
      CLOSED → OPEN:      failures >= threshold (5) within window (60s)
      OPEN → HALF-OPEN:   after cooldown period (30s)
      HALF-OPEN → CLOSED: success_count >= required (3)
      HALF-OPEN → OPEN:   any failure during half-open
  ```
- **Circuit Breakers**:
  | Component          | Threshold | Cooldown | Fallback                     |
  |-------------------|-----------|----------|-------------------------------|
  | PostgreSQL Primary | 5 / 60s   | 30s      | Read from replica, queue writes|
  | PostgreSQL Replica | 3 / 30s   | 15s      | Read from primary             |
  | Redis Cache        | 3 / 30s   | 15s      | Bypass cache, direct DB       |
  | License Server     | 3 / 60s   | 60s      | Use cached license            |
  | IDP (SSO)          | 5 / 120s  | 60s      | Local auth fallback           |
  | Webhook Delivery   | 10 / 60s  | 120s     | Queue for later delivery      |
  | External Secret Mgr| 3 / 60s   | 30s      | Use cached secrets            |
- **Acceptance Criteria**:
  - AC-1: Circuit opens within 5 seconds of threshold breach
  - AC-2: Requests fail-fast when circuit open (< 1ms response)
  - AC-3: Circuit state changes logged and alerted
  - AC-4: Fallback behavior provides degraded but functional service
  - AC-5: Circuit breaker metrics exported to Prometheus

### FR-003: Self-Healing Mechanisms
- **Mô tả**: Automated recovery cho các transient failure scenarios.
- **Self-Healing Actions**:
  ```
  SelfHealingEngine:
      Rules:
      1. DB Connection Lost:
          → Exponential backoff reconnect (1s, 2s, 4s... max 30s)
          → Max attempts: 10
          → On recovery: flush pending writes, reconcile state

      2. Redis Connection Lost:
          → Immediate fallback to no-cache mode
          → Background reconnect every 5s
          → On recovery: warm cache from DB queries

      3. Runner Goroutine Panic:
          → Catch via recover() wrapper (existing)
          → Auto-restart goroutine after 5s cooldown
          → Max restarts: 3 within 5 minutes
          → If exceeded: mark runner unhealthy, alert

      4. Memory Pressure (> 85%):
          → Force GC cycle
          → Reduce cache sizes (evict LRU entries)
          → If > 95%: graceful degrade (reject new connections)

      5. Disk Space Low (< 10%):
          → Purge old temp files
          → Compress audit logs
          → Alert operations
          → If < 5%: read-only mode

      6. Connection Pool Exhausted:
          → Reject new requests with 503 + Retry-After header
          → Kill idle connections
          → Alert operations
  ```
- **Acceptance Criteria**:
  - AC-1: DB reconnect succeeds within 60s for transient failures
  - AC-2: Redis failover to no-cache mode within 1s
  - AC-3: Runner auto-restart completes within 10s
  - AC-4: Memory pressure mitigation prevents OOM kill
  - AC-5: Self-healing events logged and metrics tracked

### FR-004: SLI/SLO Framework
- **Mô tả**: Define Service Level Indicators và Objectives theo chuẩn SRE.
- **SLIs**:
  | SLI                        | Measurement                            | Window  |
  |----------------------------|----------------------------------------|---------|
  | Availability               | % successful responses (non-5xx)       | 30 days |
  | Latency (P50)              | 50th percentile response time          | 5 min   |
  | Latency (P99)              | 99th percentile response time          | 5 min   |
  | Error Rate                 | % 5xx responses                        | 5 min   |
  | Throughput                 | Requests per second                    | 1 min   |
  | Task Success Rate          | % tasks completed successfully         | 24 hours|
  | Data Freshness             | Schema sync lag                        | 1 hour  |

- **SLOs (Financial Grade)**:
  | SLO                        | Target     | Error Budget (30 days) |
  |----------------------------|------------|------------------------|
  | Availability               | ≥ 99.95%   | 21.9 minutes           |
  | API Latency P99            | ≤ 200ms    | N/A                    |
  | Error Rate                 | ≤ 0.1%     | N/A                    |
  | Task Success Rate          | ≥ 99.9%    | 0.1% failures          |
  | Schema Sync Freshness      | ≤ 5 min    | N/A                    |

- **Error Budget Policy**:
  ```
  Error Budget Remaining:
      > 50%: Normal development velocity
      25-50%: Increased monitoring, no risky deployments
      10-25%: Feature freeze, focus on reliability
      < 10%: Incident mode, all hands on stability
  ```
- **Acceptance Criteria**:
  - AC-1: All SLIs measured and exported as Prometheus metrics
  - AC-2: SLO dashboard available in Grafana
  - AC-3: Error budget tracking with policy enforcement
  - AC-4: Monthly SLO report generated for compliance

### FR-005: Tiered Alerting & Escalation
- **Mô tả**: Multi-tier alerting system với escalation procedures.
- **Alert Tiers**:
  | Tier | Severity | Response Time | Channel                    | Example                          |
  |------|----------|---------------|----------------------------|----------------------------------|
  | P0   | Critical | 5 min         | PagerDuty + Phone          | Service down, data loss risk     |
  | P1   | High     | 15 min        | PagerDuty + Slack          | Degraded performance, failover   |
  | P2   | Medium   | 1 hour        | Slack + Email              | Single component failure         |
  | P3   | Low      | 4 hours       | Email + Dashboard          | Warning thresholds approaching   |
  | P4   | Info     | Next business | Dashboard                  | Informational, trends            |

- **Alert Rules**:
  ```yaml
  alerts:
    - name: bytebase_service_down
      severity: P0
      condition: up == 0 for 1m
      action: page_oncall

    - name: bytebase_high_error_rate
      severity: P1
      condition: error_rate > 1% for 5m
      action: page_oncall + slack

    - name: bytebase_db_connection_pool_exhausted
      severity: P1
      condition: pool_available == 0 for 30s
      action: page_oncall + slack

    - name: bytebase_circuit_breaker_open
      severity: P2
      condition: circuit_state == "open"
      action: slack + email

    - name: bytebase_replication_lag_high
      severity: P2
      condition: replication_lag_seconds > 60 for 5m
      action: slack + email

    - name: bytebase_disk_space_low
      severity: P2
      condition: disk_free_percent < 10
      action: slack + email

    - name: bytebase_memory_high
      severity: P3
      condition: memory_usage_percent > 85 for 10m
      action: email + dashboard

    - name: bytebase_error_budget_burning
      severity: P3
      condition: error_budget_remaining < 25%
      action: email + dashboard
  ```
- **Acceptance Criteria**:
  - AC-1: P0 alerts delivered within 30 seconds
  - AC-2: Escalation from P1 → P0 after 15 min without acknowledgment
  - AC-3: Alert deduplication — no duplicate alerts within 5 minutes
  - AC-4: Alert history searchable for audit compliance

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                                  | Thay đổi                                          |
|------------------------------|-----------------------------------------------|---------------------------------------------------|
| Deep Health Checker          | `backend/component/health/checker.go`         | Multi-dependency health check orchestrator        |
| PG Health Check              | `backend/component/health/pg_check.go`        | PostgreSQL health probe                           |
| Redis Health Check           | `backend/component/health/redis_check.go`     | Redis health probe                                |
| Runner Health Check          | `backend/component/health/runner_check.go`    | Runner goroutine health probe                     |
| System Health Check          | `backend/component/health/system_check.go`    | Disk, memory, CPU health probes                   |
| Circuit Breaker              | `backend/component/resilience/breaker.go`     | Generic circuit breaker implementation            |
| CB Registry                  | `backend/component/resilience/registry.go`    | Circuit breaker registry for all dependencies     |
| Self-Healing Engine          | `backend/component/resilience/healing.go`     | Automated recovery actions                        |
| SLI Collector                | `backend/metrics/sli.go`                      | SLI metric collectors                             |
| SLO Calculator               | `backend/metrics/slo.go`                      | SLO calculation and error budget tracking         |
| Alert Manager Client         | `backend/component/alert/alertmanager.go`     | Prometheus AlertManager integration               |
| Alert Rules                  | `deploy/monitoring/alerts.yaml`               | Prometheus alert rules definition                 |
| Grafana Dashboards           | `deploy/monitoring/dashboards/`               | SLI/SLO, circuit breaker dashboards               |
| Health API                   | `backend/api/v1/health_service.go`            | Health check API endpoints                        |
| Echo Routes                  | `backend/server/echo_routes.go`               | Add /healthz/deep endpoint                        |

### 3.2 Configuration

| Environment Variable          | Default     | Mô tả                                                |
|-------------------------------|-------------|-------------------------------------------------------|
| `HEALTH_CHECK_INTERVAL_SEC`  | `10`        | Deep health check interval                            |
| `CB_FAILURE_THRESHOLD`       | `5`         | Default circuit breaker failure threshold             |
| `CB_COOLDOWN_SEC`            | `30`        | Default circuit breaker cooldown period               |
| `CB_HALF_OPEN_MAX_REQUESTS`  | `3`         | Requests allowed in half-open state                   |
| `SELF_HEAL_ENABLED`          | `true`      | Enable self-healing mechanisms                        |
| `SELF_HEAL_MAX_RESTARTS`     | `3`         | Max runner restarts within window                     |
| `SLO_AVAILABILITY_TARGET`    | `99.95`     | SLO availability target percentage                    |
| `ALERT_PAGERDUTY_KEY`        | _(empty)_   | PagerDuty integration routing key                     |
| `ALERT_SLACK_WEBHOOK`        | _(empty)_   | Slack webhook URL for alerts                          |
| `MEMORY_PRESSURE_THRESHOLD`  | `85`        | Memory usage % to trigger pressure mitigation         |
| `DISK_SPACE_THRESHOLD`       | `10`        | Disk space % threshold for alerts                     |

### 3.3 Prometheus Metrics

```prometheus
# Circuit Breaker
bytebase_circuit_breaker_state{component="postgresql"} # 0=closed, 1=open, 2=half_open
bytebase_circuit_breaker_failures_total{component="postgresql"}
bytebase_circuit_breaker_transitions_total{component="postgresql",from="closed",to="open"}

# Health Check
bytebase_health_check_status{check="postgresql"} # 0=unhealthy, 1=healthy
bytebase_health_check_duration_seconds{check="postgresql"}

# SLI
bytebase_sli_availability_ratio
bytebase_sli_request_duration_seconds{quantile="0.5"}
bytebase_sli_request_duration_seconds{quantile="0.99"}
bytebase_sli_error_rate_ratio
bytebase_sli_task_success_rate_ratio

# SLO
bytebase_slo_error_budget_remaining_ratio
bytebase_slo_burn_rate_1h
bytebase_slo_burn_rate_6h

# Self-Healing
bytebase_self_healing_actions_total{action="db_reconnect"}
bytebase_self_healing_successes_total{action="db_reconnect"}
bytebase_self_healing_runner_restarts_total{runner="taskScheduler"}
```

### 3.4 Database Changes

```sql
-- Health check history for trend analysis
CREATE TABLE IF NOT EXISTS health_check_log (
    id              BIGSERIAL   PRIMARY KEY,
    check_name      TEXT        NOT NULL,
    status          TEXT        NOT NULL,  -- HEALTHY, UNHEALTHY, DEGRADED
    latency_ms      FLOAT       NOT NULL DEFAULT 0,
    message         TEXT,
    node_id         TEXT        NOT NULL,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partition by month for performance
CREATE INDEX idx_health_check_log_time
    ON health_check_log (checked_at DESC);

-- Retain 90 days of health check history
-- (cleanup via DataCleaner runner)
```

### 3.5 Frontend Changes

| Component                    | Thay đổi                                          |
|------------------------------|---------------------------------------------------|
| System health dashboard      | Admin page showing component health status        |
| Circuit breaker status       | Visual indicators for circuit states              |
| SLO dashboard widget         | Error budget burn-down chart                      |
| Alert history viewer         | Searchable alert history for audit                |

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| CR-AVAIL-001            | HA Clustering — node awareness for distributed health checks    |
| Prometheus              | Metrics collection (existing in stack)                          |
| Grafana                 | Dashboard visualization (recommended)                          |
| AlertManager            | Alert routing and deduplication                                 |
| PagerDuty/OpsGenie      | On-call escalation (optional)                                   |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                         |
|------------|-----------------------------------------------------------------|-----------------------------------------|
| TC-001     | /healthz when process running                                  | 200 OK, < 10ms                          |
| TC-002     | /readyz when all deps healthy                                  | 200 OK with component list              |
| TC-003     | /readyz when PostgreSQL unreachable                            | 503, PostgreSQL check failed            |
| TC-004     | /healthz/deep with mixed status                               | Detailed status per component           |
| TC-005     | Circuit breaker: 5 DB failures in 60s                          | Circuit opens, requests fail-fast       |
| TC-006     | Circuit breaker half-open: 3 successes                        | Circuit closes, normal operation        |
| TC-007     | Circuit breaker fallback: Redis circuit open                   | Bypass cache, direct DB query           |
| TC-008     | Self-healing: DB reconnect after transient failure             | Auto-reconnect within 60s              |
| TC-009     | Self-healing: runner panic recovery                            | Runner restarts within 10s              |
| TC-010     | Self-healing: memory pressure > 85%                            | GC triggered, cache reduced             |
| TC-011     | SLI metrics exported to Prometheus                             | All SLI metrics present                 |
| TC-012     | SLO error budget tracking                                      | Budget calculation accurate             |
| TC-013     | P0 alert delivery time                                         | Delivered within 30 seconds             |
| TC-014     | Alert deduplication                                            | No duplicate within 5 minutes           |
| TC-015     | Circuit breaker metrics in Prometheus                          | State, failures, transitions tracked    |

---

## 6. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Deep health check system                         | Sprint 1       |
| Phase 2 | Circuit breaker implementation                   | Sprint 1-2     |
| Phase 3 | Self-healing mechanisms                          | Sprint 2-3     |
| Phase 4 | SLI/SLO framework + Prometheus metrics           | Sprint 3       |
| Phase 5 | Tiered alerting + PagerDuty integration         | Sprint 4       |
| Phase 6 | Grafana dashboards + documentation              | Sprint 4       |
| Phase 7 | Load testing & validation                       | Sprint 5       |

---

## 7. Risks & Mitigations

| Risk                                         | Impact | Mitigation                                                |
|----------------------------------------------|--------|-----------------------------------------------------------|
| Health checks themselves cause load           | LOW    | Rate-limited checks, cached results (10s TTL)             |
| Circuit breaker too aggressive                | MEDIUM | Tunable thresholds, slow-start half-open                  |
| Self-healing masks underlying issues         | MEDIUM | Log all healing events, alert on repeated healing         |
| Alert fatigue from too many alerts           | HIGH   | Dedup, grouping, proper threshold tuning                  |
| False positive health failures               | MEDIUM | Multi-check validation, debounce transitions              |
