# Change Request: Uptime Kuma Health Sync Agent

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INT-005                                               |
| **Gap ID**         | G6                                                       |
| **Title**          | Uptime Kuma Health Sync Agent                            |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **External Tool**  | Uptime Kuma (louislam/uptime-kuma)                       |

---

## 1. Tổng quan

### 1.1 Mô tả
Agent tự động đồng bộ giữa Bytebase và Uptime Kuma để giám sát service health sau database changes. Agent chủ động:
- Auto-create Uptime Kuma monitors khi Bytebase instances registered
- Push Bytebase change events → Uptime Kuma maintenance windows
- Pull Uptime Kuma status → Bytebase health dashboard
- Correlate downtime events với recent Bytebase changes

### 1.2 Bối cảnh
Gap G6 yêu cầu service uptime monitoring sau chuyển user. Uptime Kuma là lightweight, dễ deploy. Agent tạo closed-loop: Bytebase change → Uptime Kuma monitoring → alert back to Bytebase.

### 1.3 Mục tiêu
- Zero-config monitor provisioning từ Bytebase
- Change-aware maintenance windows
- Downtime correlation với DB changes
- Unified health view trong Bytebase

---

## 2. Yêu cầu chức năng

### FR-001: Monitor Auto-Provisioning
- Khi DB instance added → Agent auto-creates Uptime Kuma monitors:
  - **TCP monitor**: DB port connectivity (1521, 5432, 3306, 1433, 27017)
  - **SQL monitor**: Push monitor via agent executing `SELECT 1` qua Bytebase credentials
  - **HTTP monitor**: Application health endpoints (configurable mapping)
- Monitor naming: `[Bytebase] {instance_name} - {check_type}`
- Monitor grouping: by Bytebase project/environment
- Auto-remove monitors khi instance removed

### FR-002: Maintenance Window Sync
- Khi Bytebase rollout starts → Agent creates Uptime Kuma maintenance window
  - Duration: estimated from plan + buffer
  - Affected monitors: linked to rollout target databases
  - Description: Bytebase issue title + plan details
- Khi rollout completes → maintenance window ends
- Prevents false-positive downtime alerts during planned changes

### FR-003: Health Status Pull
- Poll Uptime Kuma API → pull monitor status, response times, uptime %
- Display trong Bytebase Instance → Health tab
- Historical data: uptime trend aligned with Bytebase timeline
- SLA calculation: uptime % per service/database

### FR-004: Downtime Correlation Engine
- When Uptime Kuma detects downtime → Agent checks:
  - Was there a Bytebase change in last N minutes?
  - Which change is most likely cause? (proximity scoring)
- Auto-create diagnostic Issue nếu correlation found
- Alert: "Service X down — potentially caused by change Y"

### FR-005: Status Page Integration
- Aggregate Uptime Kuma status pages → Bytebase project dashboard
- Service status: 🟢 Operational / 🟡 Degraded / 🔴 Down
- Embed public status page URL in Bytebase notifications

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Uptime Agent Core | `backend/component/agent/uptimekuma/agent.go` | Lifecycle |
| Kuma API Client | `backend/component/agent/uptimekuma/client.go` | Uptime Kuma API |
| Monitor Provisioner | `backend/component/agent/uptimekuma/provisioner.go` | Auto-create monitors |
| Maintenance Sync | `backend/component/agent/uptimekuma/maintenance.go` | Window management |
| Status Puller | `backend/component/agent/uptimekuma/status_puller.go` | Health data pull |
| Correlation Engine | `backend/component/agent/uptimekuma/correlation.go` | Downtime analysis |
| Agent Config | `backend/component/agent/uptimekuma/config.go` | YAML config |
| Health Panel UI | `frontend/src/views/Integration/UptimeKuma/` | Status display |

### 3.1 Agent Configuration

```yaml
agent:
  name: uptime-kuma-integration
  enabled: true

uptime_kuma:
  url: "https://uptime.vnpay.vn"
  username: "admin"
  password: "${KUMA_PASSWORD}"  # from Secret Manager

monitors:
  auto_provision: true
  default_interval: 60  # seconds
  check_types:
    - tcp_port
    - sql_query  # via push monitor
  
  # Application endpoint mapping
  app_endpoints:
    - db_instance: "QR-Connector-Oracle-Prod"
      http_url: "https://qr.vnpay.vn/health"
    - db_instance: "Blockchain-PG-Prod"
      http_url: "https://blockchain.vnpay.vn/api/health"

maintenance:
  auto_create: true
  buffer_minutes: 15  # extra time after rollout

correlation:
  lookback_minutes: 30
  auto_create_issue: true
```

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Add PG instance → Kuma monitors created | TCP + SQL monitors visible |
| TC-002 | Bytebase rollout starts → maintenance window | Kuma maintenance active |
| TC-003 | Rollout completes → maintenance ends | Monitoring resumes |
| TC-004 | Service down after change → correlation | "Caused by change Y" Issue |
| TC-005 | Health status → Bytebase dashboard | 🟢/🟡/🔴 per instance |
| TC-006 | Instance removed → monitors deleted | Kuma monitors cleaned up |
| TC-007 | Kuma offline → agent buffers | Sync when recovered |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Agent core + monitor provisioning | Sprint 1-2 |
| Phase 2 | Maintenance window sync | Sprint 3 |
| Phase 3 | Status pull + correlation engine | Sprint 4 |
