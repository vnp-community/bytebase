# Change Request: Automated Failover & Disaster Recovery (DR)

| Field              | Value                                                       |
|--------------------|-------------------------------------------------------------|
| **CR ID**          | CR-AVAIL-002                                                |
| **Title**          | Automated Failover & Disaster Recovery (DR)                 |
| **Category**       | Availability / Disaster Recovery                            |
| **Priority**       | P0 — Critical                                               |
| **Status**         | Draft                                                       |
| **Created**        | 2026-05-08                                                  |
| **Author**         | VNP AI Ops Team                                             |
| **Regulatory**     | FFIEC BCM, ISO 22301 §8.3, SBV TT09/2020 Điều 10-11       |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế và triển khai hệ thống **automated failover** cho tất cả critical components của Bytebase, kết hợp **Disaster Recovery (DR) procedures** đáp ứng yêu cầu RTO/RPO của ngành tài chính.

### 1.2 Bối cảnh
Hệ thống hiện tại thiếu cơ chế failover tự động ở nhiều lớp:
- **Application layer**: Không có automatic failover khi Bytebase instance crash
- **Database layer**: Phụ thuộc hoàn toàn vào external PostgreSQL HA (không có built-in failover)
- **Runner layer**: Background runners không có failover — khi node chạy runner dies, task bị stale
- **Connection layer**: Database driver connections không reconnect automatically
- **DR**: Không có documented DR procedure, chưa có cross-site failover

### 1.3 Mục tiêu
- **RTO ≤ 30 phút** — Recovery Time Objective cho full service restoration
- **RPO ≤ 15 phút** — Recovery Point Objective cho dữ liệu
- Automated failover cho application, runner, và connection layers
- DR runbook với quarterly drill testing
- Tiered recovery strategy: Hot Standby (Tier 1) → Warm DR (Tier 2) → Cold DR (Tier 3)

### 1.4 Tiêu chuẩn áp dụng

| Standard                          | Requirement                                               |
|-----------------------------------|------------------------------------------------------------|
| FFIEC BCM Appendix J              | Recovery strategies, alternative processing                |
| ISO 22301 §8.3                    | Business continuity strategies and solutions               |
| SBV TT09/2020 — Điều 10          | Kế hoạch dự phòng và phục hồi sau thảm họa               |
| Basel III OpRisk                  | Operational resilience for critical financial services      |

---

## 2. Yêu cầu chức năng

### FR-001: Application-Level Automated Failover
- **Mô tả**: Detect application instance failure và tự động redistribute workload.
- **Failure Detection**:
  ```
  FailureDetector:
      FOR each registered node IN cluster_node:
          IF node.last_heartbeat < now() - UNHEALTHY_THRESHOLD (30s):
              markNodeUnhealthy(node)
          IF node.last_heartbeat < now() - DEAD_THRESHOLD (90s):
              initiateFailover(node)
  
  initiateFailover(deadNode):
      1. Remove deadNode from load balancer
      2. Reassign leader-elected runners to surviving nodes
      3. Retry in-progress tasks owned by deadNode (idempotent)
      4. Alert operations team (PagerDuty/OpsGenie)
      5. Log failover event in audit_log
  ```
- **Acceptance Criteria**:
  - AC-1: Failover detection time ≤ 30 seconds
  - AC-2: Runner reassignment completes within 60 seconds
  - AC-3: No task duplication — idempotent task execution
  - AC-4: Failover event logged with full context in audit trail

### FR-002: PostgreSQL Failover Integration
- **Mô tả**: Tích hợp với PostgreSQL HA solutions để tự động switch endpoint khi primary fails.
- **Supported HA Solutions**:
  | Solution              | Failover Method                              |
  |-----------------------|----------------------------------------------|
  | Patroni               | DCS-based leader election, VIP switching     |
  | PgBouncer + repmgr    | Connection pooler with automatic redirect    |
  | Cloud Managed PG      | AWS RDS Multi-AZ, GCP Cloud SQL HA           |
  | Kubernetes Operator    | CloudNativePG, Zalando PG Operator           |
- **Logic**:
  ```
  DBConnectionManager:
      primaryPool = pgxpool.New(PG_PRIMARY_URL)
      replicaPool = pgxpool.New(PG_REPLICA_URL)  // optional

      ON connectionError(pool):
          IF pool == primaryPool:
              // Attempt reconnect with exponential backoff
              FOR attempt = 1 to MAX_RECONNECT (10):
                  wait(backoff(attempt))  // 1s, 2s, 4s, 8s...
                  IF reconnect(PG_PRIMARY_URL).success:
                      BREAK
              // If all reconnects fail → check failover endpoint
              IF PG_FAILOVER_URL != "":
                  switchPrimary(PG_FAILOVER_URL)
              ELSE:
                  setServiceDegraded()
                  alertOperations()
  ```
- **Acceptance Criteria**:
  - AC-1: PostgreSQL failover transparent to application within 30s
  - AC-2: Connection pool re-established within 10s after failover
  - AC-3: Write operations paused during failover (not lost)
  - AC-4: Read operations continue via replica during primary failover

### FR-003: Runner Failover & Task Recovery
- **Mô tả**: Tasks đang chạy trên node fail được recover bởi node khác.
- **Logic**:
  ```
  TaskRecoveryRunner (runs on all nodes, leader-elected):
      EVERY 60 seconds:
          staleTasks = store.FindTasks(
              status = RUNNING,
              assigned_node NOT IN healthyNodes,
              last_updated < now() - STALE_THRESHOLD (5 min)
          )
          FOR each task IN staleTasks:
              IF task.isIdempotent():
                  store.ResetTaskStatus(task, PENDING)
                  bus.Publish(TaskRunTickle, task.ID)
                  auditLog("task_recovered", task)
              ELSE:
                  store.MarkTaskFailed(task, "node failure, manual review required")
                  alertOperations(task)
  ```
- **Acceptance Criteria**:
  - AC-1: Stale tasks detected within 5 minutes of node failure
  - AC-2: Idempotent tasks auto-recovered without manual intervention
  - AC-3: Non-idempotent tasks flagged for manual review
  - AC-4: Task recovery audit trail maintained

### FR-004: Disaster Recovery Tiers
- **Mô tả**: Implement tiered DR strategy phù hợp với classification hệ thống.
- **DR Tiers**:

  | Tier   | Strategy       | RTO      | RPO     | Use Case                    |
  |--------|----------------|----------|---------|------------------------------|
  | Tier 1 | Hot Standby    | < 5 min  | < 1 min | Core database changes        |
  | Tier 2 | Warm DR        | < 30 min | < 15 min| SQL Editor, review workflows |
  | Tier 3 | Cold DR        | < 4 hours| < 1 hour| Admin, settings, branding    |

- **Hot Standby Architecture**:
  ```
  Primary Site (Region A):
      Bytebase Cluster (3 nodes)
      PostgreSQL Primary
      Redis Primary
          │
          │ Streaming Replication (sync/async)
          │ Redis Replication
          ▼
  Standby Site (Region B):
      Bytebase Standby Cluster (2 nodes, read-only)
      PostgreSQL Standby (streaming replica)
      Redis Standby
  ```

- **Acceptance Criteria**:
  - AC-1: Tier 1 failover (hot standby) completes within 5 minutes
  - AC-2: Tier 2 failover (warm DR) completes within 30 minutes
  - AC-3: DR drill executed quarterly with documented results
  - AC-4: RPO verified via replication lag monitoring

### FR-005: DR Runbook & Automation
- **Mô tả**: Automated DR procedures với human-in-the-loop confirmation.
- **Runbook Steps**:
  ```
  DR Activation Procedure:
  1. DETECT: Monitoring alerts primary site unreachable (> 5 min)
  2. ASSESS: On-call engineer confirms genuine outage (not network blip)
  3. DECIDE: DR activation approved by 2 authorized personnel
  4. EXECUTE:
     a. Promote PG standby to primary
     b. Update DNS/GLB to point to DR site
     c. Start Bytebase cluster on DR site (full mode)
     d. Verify service health via /readyz
     e. Notify stakeholders
  5. VALIDATE: Run automated smoke tests
  6. MONITOR: Enhanced monitoring for 24 hours post-failover
  ```
- **Acceptance Criteria**:
  - AC-1: DR runbook documented and version-controlled
  - AC-2: Automated DR execution script (with manual confirmation gate)
  - AC-3: DR drill results archived for regulatory audit
  - AC-4: Failback procedure documented and tested

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                              | Thay đổi                                          |
|------------------------------|-------------------------------------------|---------------------------------------------------|
| Failure Detector             | `backend/component/cluster/detector.go`   | Heartbeat-based failure detection                 |
| Failover Manager             | `backend/component/cluster/failover.go`   | Orchestrate failover procedures                   |
| Task Recovery Runner         | `backend/runner/recovery/recovery.go`     | Stale task detection and recovery                 |
| DB Connection Failover       | `backend/store/failover.go`              | PostgreSQL connection failover handling            |
| Connection Retry             | `backend/store/retry.go`                 | Exponential backoff retry for DB connections       |
| DR Status API                | `backend/api/v1/dr_service.go`           | DR status, activation, drill management           |
| Alerting Integration         | `backend/component/alert/pagerduty.go`   | PagerDuty/OpsGenie integration for failover alerts|
| Audit — Failover Events      | `backend/component/audit/failover.go`    | Audit log entries for failover events             |

### 3.2 Configuration

| Environment Variable          | Default     | Mô tả                                                |
|-------------------------------|-------------|-------------------------------------------------------|
| `PG_FAILOVER_URL`            | _(empty)_   | PostgreSQL failover endpoint URL                      |
| `PG_RECONNECT_MAX_ATTEMPTS`  | `10`        | Maximum reconnection attempts                         |
| `PG_RECONNECT_BASE_DELAY_MS` | `1000`      | Base delay for exponential backoff                    |
| `DR_MODE`                    | `disabled`  | DR mode: disabled, hot-standby, warm-dr, cold-dr      |
| `DR_SITE_URL`                | _(empty)_   | DR site Bytebase URL                                  |
| `DR_REPLICATION_LAG_MAX_SEC` | `60`        | Max acceptable replication lag before alert           |
| `ALERT_PAGERDUTY_KEY`        | _(empty)_   | PagerDuty integration key for failover alerts         |
| `ALERT_OPSGENIE_KEY`         | _(empty)_   | OpsGenie integration key                              |
| `TASK_STALE_THRESHOLD_SEC`   | `300`       | Seconds before running task is considered stale       |

### 3.3 Database Changes

```sql
-- Task node assignment tracking
ALTER TABLE task_run ADD COLUMN IF NOT EXISTS assigned_node TEXT;
ALTER TABLE task_run ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMPTZ;

CREATE INDEX idx_task_run_assigned_node ON task_run (assigned_node)
    WHERE status IN ('RUNNING', 'PENDING');

-- DR event log
CREATE TABLE IF NOT EXISTS dr_event (
    id              BIGSERIAL   PRIMARY KEY,
    event_type      TEXT        NOT NULL,
    -- Types: FAILOVER_INITIATED, FAILOVER_COMPLETED, DR_DRILL_START,
    --        DR_DRILL_END, FAILBACK_INITIATED, FAILBACK_COMPLETED
    source_site     TEXT        NOT NULL,
    target_site     TEXT        NOT NULL,
    initiated_by    TEXT        NOT NULL,
    approved_by     TEXT[],
    status          TEXT        NOT NULL DEFAULT 'IN_PROGRESS',
    details         JSONB       NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Replication lag monitoring
CREATE TABLE IF NOT EXISTS replication_monitor (
    id              BIGSERIAL   PRIMARY KEY,
    source_site     TEXT        NOT NULL,
    target_site     TEXT        NOT NULL,
    lag_bytes       BIGINT      NOT NULL DEFAULT 0,
    lag_seconds     FLOAT       NOT NULL DEFAULT 0,
    measured_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_replication_monitor_time
    ON replication_monitor (measured_at DESC);
```

### 3.4 Frontend Changes

| Component                    | Thay đổi                                          |
|------------------------------|---------------------------------------------------|
| Service degraded banner      | Show banner when service in degraded mode         |
| DR status page (admin)       | Admin view for DR status, drill history           |
| Connection retry indicator   | Show retry count during connection recovery       |

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| CR-AVAIL-001            | HA Clustering — prerequisite for cluster-aware failover         |
| CR-AVAIL-003            | Health Monitoring — failure detection feeds failover            |
| CR-AVAIL-005            | Backup & Recovery — backup infrastructure for DR                |
| CR-LIM-001              | Distributed Cache — Redis HA for cache failover                 |
| PostgreSQL HA Solution  | Patroni, CloudNativePG, or Cloud Managed PG                     |
| PagerDuty/OpsGenie      | Alert escalation for failover events                            |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                         |
|------------|-----------------------------------------------------------------|-----------------------------------------|
| TC-001     | Kill 1 of 3 Bytebase instances                                | Failover detected ≤ 30s, traffic rerouted|
| TC-002     | Kill leader-elected runner node                                | Runner leadership transferred ≤ 60s     |
| TC-003     | PostgreSQL primary failure + Patroni failover                  | Bytebase reconnects to new primary      |
| TC-004     | Redis primary failure                                          | Fallback to DB queries, service continues|
| TC-005     | Simulate stale task (node crash during migration)              | Task detected and recovered ≤ 5 min     |
| TC-006     | Full DR activation (primary site down)                         | Service restored at DR site ≤ 30 min    |
| TC-007     | DR drill — automated execution                                | Drill completes, results logged         |
| TC-008     | Failback from DR to primary site                               | Service restored at primary site        |
| TC-009     | Replication lag exceeds threshold                              | Alert fired, reads redirected           |
| TC-010     | Concurrent failover events                                     | Only one failover executes (mutex)      |
| TC-011     | Network partition between sites                                | Split-brain prevention (fencing)        |
| TC-012     | RPO verification: check data loss after failover               | Data loss ≤ RPO target                  |

---

## 6. DR Drill Schedule (Regulatory Compliance)

| Drill Type              | Frequency   | Duration  | Scope                          |
|-------------------------|-------------|-----------|--------------------------------|
| Tabletop Exercise       | Monthly     | 2 hours   | Review procedures, roles       |
| Component Failover      | Monthly     | 1 hour    | Single component failure test  |
| Partial DR              | Quarterly   | 4 hours   | Tier 1-2 failover test         |
| Full DR                 | Semi-annual | 8 hours   | Complete site failover          |
| Surprise Drill          | Annual      | Variable  | Unannounced partial failure    |

---

## 7. Rollout Plan

| Phase   | Mô tả                                          | Timeline       |
|---------|--------------------------------------------------|----------------|
| Phase 1 | Failure detector + alerting integration          | Sprint 1-2     |
| Phase 2 | Application-level failover                       | Sprint 2-3     |
| Phase 3 | PostgreSQL failover integration                  | Sprint 3       |
| Phase 4 | Task recovery runner                             | Sprint 3-4     |
| Phase 5 | DR infrastructure + runbook                      | Sprint 4-5     |
| Phase 6 | DR drill execution + validation                  | Sprint 5-6     |
| Phase 7 | Regulatory documentation + audit preparation     | Sprint 6       |

---

## 8. Risks & Mitigations

| Risk                                         | Impact | Mitigation                                                |
|----------------------------------------------|--------|-----------------------------------------------------------|
| Split-brain during network partition         | HIGH   | STONITH/fencing, quorum-based decision                    |
| False positive failover triggers             | HIGH   | Multi-signal detection (heartbeat + probe + peer check)   |
| Data loss during async replication failover  | HIGH   | Synchronous replication for Tier 1, RPO monitoring        |
| DR site out of date                          | MEDIUM | Continuous replication monitoring + alerts                 |
| Failback procedure causes second outage      | MEDIUM | Tested failback procedure, blue-green switchover          |
| Compliance audit findings                    | HIGH   | Quarterly drills with documented evidence                 |
