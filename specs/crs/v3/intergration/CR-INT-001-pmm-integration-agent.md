# Change Request: PMM Integration Agent

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INT-001                                               |
| **Gap ID**         | G3, G5                                                   |
| **Title**          | Percona PMM Integration Agent                            |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **External Tool**  | Percona Monitoring and Management (PMM)                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Agent tự động đồng bộ bidirectional giữa Bytebase và Percona PMM Server. Agent chủ động:
- Push Bytebase instance registry → PMM (auto-register monitoring)
- Pull PMM query analytics, session data → Bytebase dashboard
- Correlate Bytebase change events với PMM performance data
- Alert từ PMM triggers → auto-create Bytebase Issues

### 1.2 Bối cảnh
PMM cung cấp deep query analytics và session monitoring mà Bytebase built-in metrics (CR-INS-005) không thay thế được. Agent này tạo cầu nối để DBA xem PMM insights ngay trong Bytebase context.

### 1.3 Mục tiêu
- Auto-provision PMM monitoring khi thêm DB instance vào Bytebase
- Embed PMM query analytics data vào Bytebase change review
- Correlate change impact với PMM metrics
- Unified alerting pipeline

---

## 2. Yêu cầu chức năng

### FR-001: Instance Auto-Registration
- Khi DBA thêm DB instance vào Bytebase → Agent tự động register instance với PMM
- PMM client provisioning: generate `pmm-admin add` commands
- Sync instance metadata (labels, groups, environment tier)
- Removal sync: unregister từ PMM khi instance bị remove khỏi Bytebase

### FR-002: Query Analytics Pull
- Poll PMM Query Analytics API (`/v1/management/QAN/...`)
- Pull top slow queries, query fingerprints, execution stats
- Display trong Bytebase Instance → Performance tab
- Cross-reference: highlight queries affected by recent schema changes

### FR-003: Session Data Sync
- Pull active session data từ PMM → Bytebase Session Dashboard (CR-INS-003)
- Enrich session data với PMM's deeper metrics (CPU, memory per query)
- Historical session trends từ PMM time-series

### FR-004: Change Impact Correlation
- When Bytebase rollout completes → push event marker to PMM
- PMM annotation API: create annotation at rollout timestamp
- Bytebase pulls before/after performance comparison from PMM
- Auto-generate change impact report

### FR-005: Alert Bridge
- Subscribe PMM alerting webhook
- PMM alert → Agent evaluates → auto-create Bytebase Issue if needed
- Alert mapping rules:
  - PMM `CriticalAlert` → Bytebase HIGH risk Issue
  - PMM `WarningAlert` → Bytebase notification only
- Bidirectional: Bytebase issue resolution → resolve PMM alert

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| PMM Agent Core | `backend/component/agent/pmm/agent.go` | Agent lifecycle |
| PMM Client | `backend/component/agent/pmm/client.go` | PMM API client |
| Instance Sync | `backend/component/agent/pmm/instance_sync.go` | Auto-registration |
| QAN Puller | `backend/component/agent/pmm/qan_puller.go` | Query analytics |
| Session Sync | `backend/component/agent/pmm/session_sync.go` | Session data |
| Alert Bridge | `backend/component/agent/pmm/alert_bridge.go` | Alert correlation |
| Agent Config | `backend/component/agent/pmm/config.go` | YAML config |
| PMM Dashboard | `frontend/src/views/Integration/PMM/` | Embedded PMM data |

### 3.1 Agent Configuration

```yaml
# bytebase-agent-pmm.yaml
agent:
  name: pmm-integration
  enabled: true
  
pmm:
  server_url: "https://pmm.vnpay.vn:443"
  api_key: "${PMM_API_KEY}"  # from Secret Manager
  verify_tls: true
  
sync:
  instance_sync_interval: 5m
  qan_pull_interval: 2m
  session_pull_interval: 30s
  alert_poll_interval: 10s

mapping:
  environment_labels:
    Production: "prod"
    Staging: "stg"
    Test: "test"
  
alert_rules:
  - pmm_severity: "critical"
    bytebase_action: "create_issue"
    bytebase_risk: "HIGH"
  - pmm_severity: "warning"
    bytebase_action: "notification"
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---|---|
| PMM API key exposure | Store in External Secret Manager |
| Network access | Agent → PMM via TLS, firewall whitelist |
| PMM data sensitivity | Filter PII before displaying in Bytebase |
| Agent compromise | Least-privilege PMM API key (read-only default) |

---

## 5. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Add PG instance in Bytebase → PMM auto-registers | PMM shows new instance |
| TC-002 | PMM QAN data → visible in Bytebase | Slow queries listed |
| TC-003 | Bytebase rollout → PMM annotation created | Annotation at timestamp |
| TC-004 | PMM critical alert → Bytebase Issue created | Issue with details |
| TC-005 | PMM server down → agent buffers events | Retry when recovered |
| TC-006 | Remove instance from Bytebase → PMM deregistered | PMM instance gone |

---

## 6. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Agent core + instance auto-registration | Sprint 1-2 |
| Phase 2 | QAN pull + session sync | Sprint 3 |
| Phase 3 | Change impact correlation | Sprint 4 |
| Phase 4 | Alert bridge + bidirectional | Sprint 5 |
