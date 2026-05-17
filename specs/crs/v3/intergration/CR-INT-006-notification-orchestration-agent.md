# Change Request: Notification Orchestration Agent

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INT-006                                               |
| **Gap ID**         | G2                                                       |
| **Title**          | Notification Orchestration Agent                         |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **External Tool**  | Slack, Email (SMTP), Microsoft Teams, PagerDuty, Jira    |

---

## 1. Tổng quan

### 1.1 Mô tả
Agent orchestration tập trung quản lý tất cả notifications từ Bytebase ecosystem (bao gồm tất cả CR-INS modules và CR-INT agents) và route tới đúng channel, đúng người, đúng thời điểm. Hoạt động như **central notification hub** thay vì mỗi module tự gửi notification riêng lẻ.

### 1.2 Bối cảnh
Bytebase native webhook (ADM-04) chỉ hỗ trợ basic webhook. Cần orchestration layer để:
- Aggregate notifications tránh alert fatigue
- Intelligent routing theo on-call schedule
- Template management cho consistent messaging
- Delivery tracking và retry logic

### 1.3 Mục tiêu
- Central notification bus cho toàn bộ Bytebase ecosystem
- Alert aggregation & deduplication
- On-call schedule awareness
- Multi-channel delivery với fallback
- Full delivery audit trail

---

## 2. Yêu cầu chức năng

### FR-001: Notification Bus
- Central event bus nhận events từ tất cả modules:
  - CR-INS-001: Policy compliance violations
  - CR-INS-002: Password expiry warnings
  - CR-INS-003: Session anomalies
  - CR-INS-004: Access review findings
  - CR-INS-005: Performance anomalies
  - CR-INS-006: Health check failures
  - CR-INS-007: Credential scan findings
  - CR-INS-008: Compliance score changes
  - CR-INT-*: Agent status events
  - Bytebase native: Approval requests, rollout events
- Event schema: `{source, severity, category, payload, timestamp, instance_context}`

### FR-002: Routing Rules Engine
- **Rule-based routing**: Route events to channels based on:
  - Severity: critical/high/medium/low/info
  - Category: security, compliance, performance, operational
  - Instance context: environment, project, engine
  - Time window: business hours vs off-hours
  - On-call schedule
- **Escalation chains**: If no acknowledgment within N minutes → escalate

### FR-003: Channel Adapters
| Channel | Adapter | Features |
|---|---|---|
| Slack | Bot API + Webhook | Interactive buttons (Approve/Snooze), threads, rich formatting |
| Email | SMTP + templates | HTML templates, attachments, cc/bcc support |
| Microsoft Teams | Webhook + Adaptive Cards | Rich cards, action buttons |
| PagerDuty | Events API v2 | Incident creation, acknowledge, resolve sync |
| Jira | REST API | Auto-create tickets, link to Bytebase Issues |
| Webhook | Generic HTTP | Custom payload templates for any endpoint |
| In-App | Bytebase notification | Native notification bell + notification center |

### FR-004: Alert Aggregation & Deduplication
- **Time-window aggregation**: Group related events within 5-minute window
  - "3 password expiry warnings" instead of 3 separate notifications
- **Deduplication**: Same event from same source → suppress duplicate
- **Digest mode**: Daily/weekly summary email instead of individual notifications
- **Alert fatigue prevention**: Auto-suppress if > N alerts from same source in 1 hour

### FR-005: Template Management
- Shared notification templates cho tất cả channels
- Template variables: `{{severity}}`, `{{source}}`, `{{instance}}`, `{{message}}`, `{{action_url}}`
- Per-channel formatting: Slack blocks, HTML email, Teams adaptive cards
- Template versioning + preview
- i18n support: Vietnamese + English

### FR-006: Delivery Tracking & Audit
- Track delivery status per notification: QUEUED → SENDING → DELIVERED → READ → ACKNOWLEDGED
- Retry logic: exponential backoff, max 3 retries
- Fallback channels: primary Slack → fallback Email → fallback PagerDuty
- Full audit log: who received what, when, acknowledgment timestamps

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Notification Bus | `backend/component/agent/notifier/bus.go` | Event bus |
| Routing Engine | `backend/component/agent/notifier/router.go` | Rule evaluation |
| Aggregator | `backend/component/agent/notifier/aggregator.go` | Dedup & grouping |
| Slack Adapter | `backend/component/agent/notifier/slack.go` | Slack Bot API |
| Email Adapter | `backend/component/agent/notifier/email.go` | SMTP + templates |
| Teams Adapter | `backend/component/agent/notifier/teams.go` | MS Teams |
| PagerDuty Adapter | `backend/component/agent/notifier/pagerduty.go` | PD Events API |
| Jira Adapter | `backend/component/agent/notifier/jira.go` | Jira REST API |
| Template Engine | `backend/component/agent/notifier/template.go` | Rendering |
| Delivery Tracker | `backend/component/agent/notifier/tracker.go` | Status tracking |
| Notification API | `backend/api/v1/notification_orchestrator_service.go` | API |
| Notification Center | `frontend/src/views/NotificationCenter/` | UI |
| Routing Rules UI | `frontend/src/views/NotificationCenter/RoutingRules.vue` | Rule config |
| Template Editor | `frontend/src/views/NotificationCenter/Templates.vue` | Template mgmt |

### 3.1 Agent Configuration

```yaml
agent:
  name: notification-orchestrator
  enabled: true

channels:
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
    default_channel: "#dba-alerts"
    channels:
      critical: "#dba-critical"
      security: "#security-alerts"
  email:
    smtp_host: "smtp.vnpay.vn"
    smtp_port: 587
    from: "bytebase-alerts@vnpay.vn"
    templates_dir: "/etc/bytebase/email-templates"
  pagerduty:
    integration_key: "${PD_INTEGRATION_KEY}"
    severity_mapping:
      critical: "critical"
      high: "error"
      medium: "warning"
  jira:
    url: "https://jira.vnpay.vn"
    api_token: "${JIRA_TOKEN}"
    project_key: "DBOPS"

routing:
  rules:
    - match: {severity: "critical"}
      channels: ["slack:critical", "pagerduty", "email:dba-team@vnpay.vn"]
    - match: {severity: "high", category: "security"}
      channels: ["slack:security", "email:security-team@vnpay.vn"]
    - match: {severity: "medium"}
      channels: ["slack:default", "in-app"]
    - match: {severity: "low"}
      channels: ["in-app"]
  
  escalation:
    - after: 15m
      channels: ["email:dba-lead@vnpay.vn"]
    - after: 30m
      channels: ["pagerduty", "email:cto@vnpay.vn"]

aggregation:
  window: 5m
  digest_schedule: "0 8 * * 1"  # weekly Monday 8am
```

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | Critical event → Slack + PD + Email | All 3 channels receive |
| TC-002 | 5 events in 2 min → aggregated to 1 | Single notification |
| TC-003 | No ack in 15m → escalation | Manager notified |
| TC-004 | Slack down → fallback to Email | Email delivered |
| TC-005 | Template renders correctly per channel | Rich formatting |
| TC-006 | Weekly digest sent Monday 8am | Summary email received |
| TC-007 | Delivery audit trail complete | All statuses tracked |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Bus + Slack + Email adapters | Sprint 1-2 |
| Phase 2 | Routing engine + aggregation | Sprint 3 |
| Phase 3 | PagerDuty + Jira + escalation | Sprint 4 |
| Phase 4 | Templates + delivery tracking | Sprint 5 |
