# Change Request: Multi-Region Active-Standby & Geo-Redundancy

| Field              | Value                                                       |
|--------------------|-------------------------------------------------------------|
| **CR ID**          | CR-AVAIL-006                                                |
| **Title**          | Multi-Region Active-Standby & Geo-Redundancy                |
| **Category**       | Availability / Infrastructure                               |
| **Priority**       | P1 — High                                                   |
| **Status**         | Draft                                                       |
| **Created**        | 2026-05-08                                                  |
| **Author**         | VNP AI Ops Team                                             |
| **Regulatory**     | FFIEC BCM, ISO 22301 §8.3, SBV TT09/2020 Điều 10-11       |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai kiến trúc **Multi-Region Active-Standby** với **geo-redundancy** cho Bytebase, đảm bảo khả năng phục hồi khi toàn bộ datacenter/region gặp sự cố — yêu cầu bắt buộc cho hệ thống thông tin cấp độ 4+ theo phân loại của Ngân hàng Nhà nước.

### 1.2 Bối cảnh
Hệ thống hiện tại được thiết kế cho single-site deployment:
- Tất cả components (app, DB, cache) nằm trong cùng một datacenter
- Không có geographic redundancy
- DR phụ thuộc hoàn toàn vào backup restore (cold DR)
- Không đáp ứng yêu cầu "datacenter-level resilience" của ngành tài chính
- Regulatory gap: SBV yêu cầu hệ thống core phải có DR site cách ≥ 30km

### 1.3 Mục tiêu
- **Active-Standby** architecture với automatic geo-failover
- **Standby site** cách primary ≥ 30km (SBV requirement)
- **Warm standby** RTO ≤ 15 phút cho Tier 1 failover
- **Data replication** lag ≤ 5 phút cho cross-region
- **Multi-region DNS** failover với health checks
- **Configuration-as-Code** cho multi-site deployment

### 1.4 Tiêu chuẩn áp dụng

| Standard                          | Requirement                                               |
|-----------------------------------|------------------------------------------------------------|
| FFIEC BCM                         | Geographic redundancy, alternative site processing         |
| ISO 22301 §8.3                    | BC strategies including geographic diversity               |
| SBV TT09/2020 — Điều 10          | DR site cách primary ≥ 30km cho hệ thống core             |
| SBV TT09/2020 — Điều 11          | Dự phòng dữ liệu tại site phụ                             |
| Basel III OpRisk                  | Geographic resilience for critical operations              |

---

## 2. Yêu cầu chức năng

### FR-001: Multi-Region Topology Management
- **Mô tả**: Define và manage multi-region deployment topology.
- **Topology Model**:
  ```
  RegionTopology:
      primaryRegion:
          name: "region-a-hanoi"
          role: PRIMARY
          location: "Hanoi DC1"
          components:
              bytebase: { replicas: 3, mode: ACTIVE }
              postgresql: { mode: PRIMARY, syncReplica: true }
              redis: { mode: PRIMARY, sentinel: true }

      standbyRegion:
          name: "region-b-hcmc"
          role: STANDBY
          location: "HCMC DC2"  # ≥ 30km from primary
          components:
              bytebase: { replicas: 2, mode: STANDBY_READ }
              postgresql: { mode: STANDBY, streaming: true }
              redis: { mode: REPLICA, sentinel: true }

      drRegion (optional):
          name: "region-c-danang"
          role: DR
          location: "DaNang DC3"
          components:
              bytebase: { replicas: 1, mode: COLD_STANDBY }
              postgresql: { mode: STANDBY, asyncReplica: true }
              redis: { mode: NONE }
  ```
- **Acceptance Criteria**:
  - AC-1: Topology configurable via YAML/environment
  - AC-2: Region status queryable via admin API
  - AC-3: Minimum 2 regions (primary + standby) for financial compliance
  - AC-4: Distance between primary and standby ≥ 30km

### FR-002: Cross-Region Data Replication
- **Mô tả**: Continuous data replication giữa primary và standby regions.
- **Replication Architecture**:
  ```
  Replication Streams:
  
  1. PostgreSQL Streaming Replication (primary → standby):
     - Mode: Synchronous (Tier 1 data)
     - Lag target: ≤ 5 seconds
     - Monitor: pg_stat_replication
  
  2. PostgreSQL Async Replication (primary → DR):
     - Mode: Asynchronous
     - Lag target: ≤ 5 minutes
     - Monitor: pg_stat_replication
  
  3. Redis Replication (primary → standby):
     - Mode: Redis Sentinel with replication
     - Lag target: ≤ 1 second
  
  4. Backup Replication (primary → standby → DR):
     - Full backups replicated within 1 hour
     - WAL archives streamed continuously
  ```
- **Replication Lag Monitoring**:
  ```
  ReplicationMonitor:
      EVERY 30 seconds:
          pgLag = query("SELECT extract(epoch from replay_lag) FROM pg_stat_replication")
          redisLag = redis.INFO("replication")

          store.UpdateReplicationStatus({
              pgLagSeconds:    pgLag,
              redisLagSeconds: redisLag,
              isHealthy:       pgLag < threshold AND redisLag < threshold,
          })

          IF pgLag > warningThreshold (30s):
              alert("replication_lag_warning", pgLag)
          IF pgLag > criticalThreshold (300s):
              alert("replication_lag_critical", pgLag)
  ```
- **Acceptance Criteria**:
  - AC-1: PG streaming replication lag ≤ 5 seconds (sync mode)
  - AC-2: PG async replication lag ≤ 5 minutes
  - AC-3: Replication lag monitored and alerted
  - AC-4: Data consistency verified daily (checksum comparison)

### FR-003: Geo-Failover Automation
- **Mô tả**: Automated failover từ primary region sang standby region.
- **Failover Procedure**:
  ```
  GeoFailover:
      TRIGGER conditions (ANY):
          - Primary region health checks failing > 5 minutes
          - Manual activation by 2 authorized personnel
          - Pre-scheduled DR drill

      PROCEDURE:
      1. VERIFY primary truly unavailable (multi-probe check)
         - Health probe from standby → primary
         - Health probe from external monitor → primary
         - DNS resolution check
         - Cross-region network check

      2. PROMOTE standby PostgreSQL to primary
         - pg_ctl promote (streaming replica → primary)
         - Verify promotion successful
         - Verify data consistency (last LSN check)

      3. UPDATE Bytebase standby mode → active
         - Switch cluster mode from STANDBY_READ → ACTIVE
         - Enable write operations
         - Start background runners (leader election)

      4. UPDATE DNS/GLB routing
         - DNS failover: TTL=60s, health-checked
         - GLB policy update to route to standby region
         - Wait for DNS propagation (≤ 5 minutes)

      5. VERIFY service health
         - Run smoke tests on new primary
         - Verify API responsiveness
         - Check data integrity

      6. NOTIFY stakeholders
         - Alert operations team
         - Update status page
         - Notify regulatory contacts (if required)

      7. MONITOR enhanced
         - 24-hour enhanced monitoring post-failover
         - Prepare failback plan
  ```
- **Acceptance Criteria**:
  - AC-1: Geo-failover completes within 15 minutes (warm standby)
  - AC-2: Data loss ≤ replication lag at time of failure
  - AC-3: Multi-probe verification prevents false failovers
  - AC-4: Failover events logged in audit trail with full timeline

### FR-004: Standby Read Access (Active-Standby)
- **Mô tả**: Standby site xử lý read-only requests, giảm load primary.
- **Read Routing**:
  ```
  Standby Read Mode:
      Allowed operations:
          - SQL Editor read-only queries (managed instances)
          - Schema diagram viewing
          - Audit log reading
          - Dashboard aggregation
          - Project/instance browsing

      Blocked operations (redirect to primary):
          - Schema changes (Issue/Plan/Rollout)
          - IAM policy changes
          - Instance creation/modification
          - Settings changes
          - Any write operation

      Implementation:
          IF region.role == STANDBY_READ:
              interceptor.enforcedReadOnly = true
              FOR each request:
                  IF request.isWrite():
                      RETURN redirect(primaryRegion.endpoint, request)
                  ELSE:
                      process(request)
  ```
- **Acceptance Criteria**:
  - AC-1: Read operations processed locally at standby site
  - AC-2: Write operations transparently redirected to primary
  - AC-3: Read performance at standby ≤ 110% of primary
  - AC-4: Standby handles ≥ 30% of total read traffic

### FR-005: Failback Procedure
- **Mô tả**: Procedure để khôi phục primary site và failback từ standby.
- **Failback Steps**:
  ```
  Failback Procedure (after primary site restored):
      1. REBUILD primary PostgreSQL as streaming replica of current primary
      2. WAIT for full sync (pgbasebackup + catch-up)
      3. VERIFY data consistency between sites
      4. SCHEDULE failback window (maintenance)
      5. EXECUTE:
         a. Stop writes briefly (read-only mode cluster-wide)
         b. Ensure replication fully caught up (lag = 0)
         c. Promote original primary
         d. Demote standby back to replica
         e. Switch DNS/GLB back to original primary
         f. Re-enable writes
      6. VERIFY service health
      7. RESUME enhanced monitoring for 24 hours
  ```
- **Acceptance Criteria**:
  - AC-1: Failback write pause ≤ 5 minutes
  - AC-2: Zero data loss during failback
  - AC-3: Failback tested during quarterly DR drills
  - AC-4: Failback procedure documented in runbook

### FR-006: Multi-Region Configuration Management
- **Mô tả**: Centralized configuration management cho multi-region deployment.
- **Configuration Sync**:
  ```
  RegionConfigManager:
      Configuration types:
      1. Shared config (synced across regions):
         - Database schema (via self-migration)
         - IAM policies
         - Project settings
         - Environment definitions

      2. Region-specific config (per-region):
         - Endpoint URLs
         - Instance connection details
         - Cache configuration
         - Network settings

      3. Secret management:
         - Per-region encryption keys
         - Database credentials
         - API keys

      Config storage:
          Shared config → PostgreSQL (replicated)
          Region config → Environment variables / ConfigMap
          Secrets → External Secret Manager (HashiCorp Vault)
  ```
- **Acceptance Criteria**:
  - AC-1: Shared config propagated to all regions via replication
  - AC-2: Region-specific config isolated per region
  - AC-3: Secrets never replicated in plaintext
  - AC-4: Config drift detection between regions

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                                  | Thay đổi                                          |
|------------------------------|-----------------------------------------------|---------------------------------------------------|
| Region Manager               | `backend/component/region/manager.go`         | Multi-region topology management                  |
| Region Config                | `backend/component/region/config.go`          | Region configuration parsing                      |
| Replication Monitor          | `backend/runner/replication/monitor.go`        | Cross-region replication lag monitoring            |
| Geo-Failover Engine          | `backend/component/region/failover.go`        | Automated geo-failover orchestration              |
| Standby Read Interceptor     | `backend/api/v1/standby_interceptor.go`       | Read-only enforcement for standby mode            |
| Write Redirector             | `backend/api/v1/write_redirector.go`          | Redirect writes to primary region                 |
| Failback Controller          | `backend/component/region/failback.go`        | Failback procedure orchestration                  |
| Config Drift Detector        | `backend/runner/config/drift_detector.go`     | Detect config drift between regions               |
| Region Health Probes         | `backend/component/region/health.go`          | Cross-region health probing                       |
| Region Admin API             | `backend/api/v1/region_service.go`            | Region management API endpoints                   |

### 3.2 Configuration

| Environment Variable          | Default         | Mô tả                                                |
|-------------------------------|-----------------|-------------------------------------------------------|
| `REGION_NAME`                | _(required)_    | Current region identifier                            |
| `REGION_ROLE`                | `PRIMARY`       | Region role: PRIMARY, STANDBY_READ, DR, COLD_STANDBY |
| `REGION_PRIMARY_URL`         | _(empty)_       | Primary region Bytebase URL (for standby)            |
| `REGION_STANDBY_URL`        | _(empty)_       | Standby region Bytebase URL (for primary)            |
| `REGION_DR_URL`             | _(empty)_       | DR region Bytebase URL                               |
| `REGION_FAILOVER_ENABLED`   | `false`         | Enable automated geo-failover                        |
| `REGION_FAILOVER_THRESHOLD` | `300`           | Seconds of primary failure before auto-failover      |
| `REGION_READ_REDIRECT`      | `true`          | Enable read traffic at standby                       |
| `REGION_WRITE_REDIRECT`     | `true`          | Redirect writes from standby to primary              |
| `REPLICATION_LAG_WARN_SEC`  | `30`            | Replication lag warning threshold                    |
| `REPLICATION_LAG_CRIT_SEC`  | `300`           | Replication lag critical threshold                   |
| `DNS_FAILOVER_PROVIDER`     | _(empty)_       | DNS provider: route53, cloudflare, cloudns           |
| `DNS_FAILOVER_DOMAIN`       | _(empty)_       | Domain for DNS-based failover                        |
| `DNS_HEALTH_CHECK_PATH`     | `/readyz`       | Health check path for DNS provider                   |

### 3.3 Database Changes

```sql
-- Region registry
CREATE TABLE IF NOT EXISTS region_registry (
    region_name     TEXT        PRIMARY KEY,
    role            TEXT        NOT NULL,
    -- Roles: PRIMARY, STANDBY_READ, DR, COLD_STANDBY
    endpoint_url    TEXT        NOT NULL,
    location        TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'INITIALIZING',
    -- Status: INITIALIZING, ACTIVE, STANDBY, FAILING_OVER, FAILED_OVER, MAINTENANCE
    
    -- Replication status
    pg_repl_lag_sec FLOAT       NOT NULL DEFAULT 0,
    redis_repl_lag_sec FLOAT    NOT NULL DEFAULT 0,
    last_health_check TIMESTAMPTZ,
    health_status   TEXT        NOT NULL DEFAULT 'UNKNOWN',
    
    -- Failover tracking
    last_failover_at TIMESTAMPTZ,
    failover_count  INT         NOT NULL DEFAULT 0,
    
    -- Metadata
    metadata        JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Geo-failover event log
CREATE TABLE IF NOT EXISTS geo_failover_event (
    id              BIGSERIAL   PRIMARY KEY,
    event_type      TEXT        NOT NULL,
    -- Types: GEO_FAILOVER_START, GEO_FAILOVER_COMPLETE,
    --        GEO_FAILBACK_START, GEO_FAILBACK_COMPLETE,
    --        GEO_DRILL_START, GEO_DRILL_COMPLETE
    source_region   TEXT        NOT NULL,
    target_region   TEXT        NOT NULL,
    trigger         TEXT        NOT NULL,
    -- Triggers: AUTOMATIC, MANUAL, DRILL
    initiated_by    TEXT        NOT NULL,
    approved_by     TEXT[],
    
    -- Timing
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    duration_sec    FLOAT,
    
    -- Data integrity
    source_lsn      TEXT,  -- PostgreSQL LSN at failover point
    target_lsn      TEXT,
    data_loss_sec   FLOAT,  -- Estimated data loss in seconds
    
    -- Results
    status          TEXT        NOT NULL DEFAULT 'IN_PROGRESS',
    verification    JSONB       NOT NULL DEFAULT '{}',
    details         JSONB       NOT NULL DEFAULT '{}',
    error_message   TEXT,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3.4 DNS Failover Configuration (Route 53 Example)

```json
{
  "HostedZoneId": "Z1234567890",
  "ChangeBatch": {
    "Changes": [
      {
        "Action": "UPSERT",
        "ResourceRecordSet": {
          "Name": "bytebase.bank.vn",
          "Type": "A",
          "SetIdentifier": "primary-hanoi",
          "Failover": "PRIMARY",
          "HealthCheckId": "hc-primary-001",
          "AliasTarget": {
            "DNSName": "primary-lb.region-a.elb.amazonaws.com",
            "EvaluateTargetHealth": true
          }
        }
      },
      {
        "Action": "UPSERT",
        "ResourceRecordSet": {
          "Name": "bytebase.bank.vn",
          "Type": "A",
          "SetIdentifier": "standby-hcmc",
          "Failover": "SECONDARY",
          "HealthCheckId": "hc-standby-001",
          "AliasTarget": {
            "DNSName": "standby-lb.region-b.elb.amazonaws.com",
            "EvaluateTargetHealth": true
          }
        }
      }
    ]
  }
}
```

### 3.5 Prometheus Metrics

```prometheus
# Region Status
bytebase_region_status{region="region-a-hanoi"}  # 0=down, 1=standby, 2=active
bytebase_region_role{region="region-a-hanoi"}     # enum: primary, standby, dr

# Replication
bytebase_replication_lag_seconds{source="region-a", target="region-b", type="postgresql"}
bytebase_replication_lag_seconds{source="region-a", target="region-b", type="redis"}
bytebase_replication_status{source="region-a", target="region-b"}  # 0=broken, 1=lagging, 2=synced

# Geo-Failover
bytebase_geo_failover_total{trigger="automatic"}
bytebase_geo_failover_duration_seconds
bytebase_geo_failover_data_loss_seconds

# Cross-Region Health
bytebase_cross_region_latency_ms{source="region-a", target="region-b"}
bytebase_cross_region_health{source="region-a", target="region-b"}  # 0=unreachable, 1=healthy
```

### 3.6 Frontend Changes

| Component                    | Thay đổi                                          |
|------------------------------|---------------------------------------------------|
| Region status indicator      | Header badge showing current region + role        |
| Multi-region admin page      | Region topology, replication status, failover UI  |
| Read-only mode banner        | Banner when accessing standby in read-only mode   |
| Write redirect indicator     | Toast notification when write redirected          |
| Failover progress page       | Real-time failover progress for admins            |

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                                          |
|-------------------------|-----------------------------------------------------------------|
| CR-AVAIL-001            | HA Clustering — cluster mode prerequisite                       |
| CR-AVAIL-002            | Failover — application-level failover within region             |
| CR-AVAIL-005            | Backup & Recovery — cross-region backup replication             |
| PostgreSQL 14+          | Streaming replication, logical replication                      |
| Redis Sentinel          | Redis cross-region replication                                  |
| DNS Provider             | Route 53, Cloudflare, CloudNS for DNS failover                 |
| Network                 | Low-latency cross-region connectivity (< 50ms)                  |

---

## 5. Test Cases

| Test ID    | Mô tả                                                          | Expected Result                         |
|------------|-----------------------------------------------------------------|-----------------------------------------|
| TC-001     | Primary and standby regions healthy                            | Both regions report healthy             |
| TC-002     | Replication lag monitoring                                     | Lag visible, within thresholds          |
| TC-003     | Replication lag exceeds warning threshold                      | P2 alert fired                          |
| TC-004     | Replication lag exceeds critical threshold                     | P1 alert fired                          |
| TC-005     | Standby read-only access                                       | Read queries succeed, writes redirected |
| TC-006     | Write redirect from standby to primary                        | Write processed at primary              |
| TC-007     | Geo-failover: primary site down                               | Standby promoted within 15 min         |
| TC-008     | Geo-failover: data loss measurement                            | Data loss ≤ replication lag             |
| TC-009     | Geo-failover: DNS switch                                      | Traffic routed to new primary          |
| TC-010     | Failback after primary restored                                | Original primary restored, ≤ 5 min pause|
| TC-011     | Config drift detection                                         | Drift detected and alerted             |
| TC-012     | Cross-region health check                                      | Latency and health measured            |
| TC-013     | Multi-region DR drill                                          | Full drill completed, documented       |
| TC-014     | False failover prevention                                      | Multi-probe prevents false positive    |
| TC-015     | Concurrent access during failover                              | Graceful degradation, no crashes       |

---

## 6. Multi-Region Deployment Architecture

```
                        Internet
                           │
                    ┌──────┴──────┐
                    │  Global DNS │
                    │ (Route 53)  │
                    │ Health-based│
                    │  failover   │
                    └──────┬──────┘
                           │
               ┌───────────┼───────────┐
               │                       │
        ┌──────▼──────┐         ┌──────▼──────┐
        │  Region A   │         │  Region B   │
        │ (Hanoi DC1) │   ≥30km │ (HCMC DC2)  │
        │  PRIMARY    │◄───────►│  STANDBY    │
        ├─────────────┤  Sync   ├─────────────┤
        │             │  Repl.  │             │
        │ ┌─────────┐ │         │ ┌─────────┐ │
        │ │ BB × 3  │ │         │ │ BB × 2  │ │
        │ │ (Active) │ │         │ │ (Read)  │ │
        │ └─────────┘ │         │ └─────────┘ │
        │             │         │             │
        │ ┌─────────┐ │         │ ┌─────────┐ │
        │ │ PG      │ │ ──WAL──►│ │ PG      │ │
        │ │ Primary │ │  Stream │ │ Standby │ │
        │ └─────────┘ │         │ └─────────┘ │
        │             │         │             │
        │ ┌─────────┐ │         │ ┌─────────┐ │
        │ │ Redis   │ │ ──────►│ │ Redis   │ │
        │ │ Primary │ │  Repl.  │ │ Replica │ │
        │ └─────────┘ │         │ └─────────┘ │
        └─────────────┘         └─────────────┘
```

---

## 7. Rollout Plan

| Phase   | Mô tả                                                | Timeline       |
|---------|--------------------------------------------------------|----------------|
| Phase 1 | Region topology model + configuration               | Sprint 1       |
| Phase 2 | Cross-region PostgreSQL streaming replication        | Sprint 1-2     |
| Phase 3 | Cross-region Redis replication (Sentinel)            | Sprint 2       |
| Phase 4 | Replication monitoring + alerting                    | Sprint 2-3     |
| Phase 5 | Standby read access + write redirect                | Sprint 3       |
| Phase 6 | Geo-failover automation                              | Sprint 4-5     |
| Phase 7 | DNS failover integration                             | Sprint 5       |
| Phase 8 | Failback procedure                                   | Sprint 5-6     |
| Phase 9 | Multi-region DR drill                                | Sprint 6       |
| Phase 10| Regulatory documentation + compliance reporting      | Sprint 6-7     |

---

## 8. Risks & Mitigations

| Risk                                         | Impact  | Mitigation                                                |
|----------------------------------------------|---------|-----------------------------------------------------------|
| Cross-region latency impacts sync replication| HIGH    | Tunable sync/async per data criticality                   |
| Split-brain during network partition         | CRITICAL| Quorum-based fencing, manual confirmation gate             |
| DNS propagation delay (TTL)                  | MEDIUM  | Low TTL (60s), GLB for instant failover                   |
| Cross-region bandwidth costs                 | MEDIUM  | Compression, WAL-only replication (minimal bandwidth)     |
| Regulatory compliance verification           | HIGH    | Quarterly drills, documented evidence, audit trail        |
| Standby site drift from primary             | MEDIUM  | Config drift detection, daily consistency checks          |
| False geo-failover due to network blip      | HIGH    | Multi-probe verification (≥3 independent checks)         |

---

## 9. Compliance Checklist

| #  | Requirement (SBV TT09/2020)                    | Status  | Evidence                         |
|----|------------------------------------------------|---------|------------------------------------|
| 1  | DR site cách primary ≥ 30km                   | PENDING | Site location documentation       |
| 2  | Dữ liệu được replicate liên tục              | PENDING | Replication monitoring dashboard  |
| 3  | RTO ≤ 30 phút cho hệ thống cấp 4             | PENDING | TC-007, DR drill results          |
| 4  | RPO ≤ 15 phút cho dữ liệu critical           | PENDING | TC-008, replication lag metrics   |
| 5  | DR drill thực hiện tối thiểu 1 lần/năm        | PENDING | Drill schedule, results archive   |
| 6  | Quy trình failback được document               | PENDING | Runbook, TC-010                   |
| 7  | Giám sát replication liên tục                  | PENDING | TC-002, TC-003, TC-004            |
| 8  | Phân quyền failover (≥2 người authorized)     | PENDING | Approval workflow                 |
