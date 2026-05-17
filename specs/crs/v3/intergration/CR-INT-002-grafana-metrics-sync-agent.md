# Change Request: Grafana Metrics Sync Agent

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INT-002                                               |
| **Gap ID**         | G5, G8                                                   |
| **Title**          | Grafana Metrics Sync Agent                               |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **External Tool**  | Grafana + Prometheus + DB Exporters                      |

---

## 1. Tổng quan

### 1.1 Mô tả
Agent tự động tích hợp bidirectional giữa Bytebase và Grafana/Prometheus stack:
- **Push**: Bytebase events → Grafana annotations, Prometheus custom metrics
- **Pull**: Grafana dashboard data → Bytebase compliance dashboard
- **Auto-provision**: Tự động tạo Grafana dashboards, Prometheus targets, alert rules khi Bytebase instances thay đổi

### 1.2 Mục tiêu
- Zero-config Grafana dashboard provisioning khi thêm DB instance
- Bytebase change events as Grafana annotations
- Compliance metrics exposed to Grafana
- Unified alert routing: Grafana alerts → Bytebase Issues

---

## 2. Yêu cầu chức năng

### FR-001: Grafana Dashboard Auto-Provisioning
- Khi DB instance added → Agent auto-creates Grafana dashboard:
  - Template dashboards per engine type (Oracle, PG, MySQL, MSSQL, MongoDB)
  - Panels: connections, QPS, cache hit, slow queries, password expiry countdown
  - Variables linked to Bytebase instance metadata
- Dashboard folder structure mirrors Bytebase project/environment hierarchy
- Dashboard update/removal synced with Bytebase changes

### FR-002: Prometheus Target Auto-Registration
- Auto-generate Prometheus `file_sd_configs` targets:
  - Map Bytebase instances → exporter targets
  - Labels from Bytebase: project, environment, engine, group
- Custom recording rules generation cho VNPAY-specific metrics:
  - `vnpay_db_password_days_until_expiry`
  - `vnpay_db_active_sessions_old_user`
  - `vnpay_db_compliance_score`

### FR-003: Change Event Annotations
- Bytebase rollout events → Grafana Annotations API
- Annotation data: issue title, SQL type, affected databases, approver
- Visible on all relevant dashboards at rollout timestamp
- Enables visual correlation: performance change ↔ DB change

### FR-004: Compliance Metrics Bridge
- Export Bytebase compliance data (CR-INS-008) as Prometheus metrics
- Grafana dashboards consume these metrics for compliance panels
- Metrics exposed:
  - `bytebase_compliance_score{domain="password_policy", instance="..."}`
  - `bytebase_password_expiry_days{user="...", instance="..."}`
  - `bytebase_access_violations_total{instance="..."}`

### FR-005: Alert Routing Bridge
- Grafana alerting rules → Bytebase webhook receiver
- Agent parses Grafana alert payload → auto-create Bytebase Issue or notification
- Alert routing rules configurable per severity/label

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Grafana Agent Core | `backend/component/agent/grafana/agent.go` | Agent lifecycle |
| Grafana API Client | `backend/component/agent/grafana/client.go` | Grafana HTTP API |
| Dashboard Provisioner | `backend/component/agent/grafana/dashboard.go` | Auto-create dashboards |
| Prometheus SD Writer | `backend/component/agent/grafana/prom_sd.go` | file_sd_configs |
| Annotation Pusher | `backend/component/agent/grafana/annotation.go` | Change annotations |
| Metrics Exporter | `backend/component/agent/grafana/metrics_bridge.go` | Compliance metrics |
| Alert Receiver | `backend/component/agent/grafana/alert_receiver.go` | Webhook handler |
| Agent Config | `backend/component/agent/grafana/config.go` | YAML config |
| Grafana UI Panel | `frontend/src/views/Integration/Grafana/` | Config & status |

### 3.1 Dashboard Template Example

```json
{
  "dashboard": {
    "title": "{{instance_name}} - {{engine_type}} Overview",
    "tags": ["bytebase-auto", "{{environment}}", "{{project}}"],
    "panels": [
      {
        "title": "Active Connections",
        "type": "timeseries",
        "targets": [{"expr": "pg_stat_activity_count{instance=\"{{exporter_target}}\"}"}]
      },
      {
        "title": "Password Expiry Countdown",
        "type": "gauge",
        "targets": [{"expr": "vnpay_db_password_days_until_expiry{instance=\"{{instance_name}}\"}"}]
      },
      {
        "title": "Bytebase Changes",
        "type": "annotations",
        "datasource": "-- Grafana --"
      }
    ]
  }
}
```

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Add PG instance → Grafana dashboard created | Dashboard visible |
| TC-002 | Bytebase rollout → Grafana annotation | Annotation on timeline |
| TC-003 | Compliance score → Prometheus metric | Scrapeable metric |
| TC-004 | Grafana alert → Bytebase Issue | Issue created |
| TC-005 | Remove instance → dashboard deleted | Dashboard removed |
| TC-006 | Grafana offline → agent buffers | Events replayed on recovery |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Dashboard provisioning + Prometheus SD | Sprint 1-2 |
| Phase 2 | Change annotations + compliance metrics | Sprint 3 |
| Phase 3 | Alert routing bridge | Sprint 4 |
