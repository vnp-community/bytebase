# Change Request: Support — Dedicated SLA

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-004                                               |
| **Feature ID**     | PRICING-SUPPORT                                          |
| **Title**          | Enterprise Support — Dedicated SLA                       |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống **Dedicated SLA Support** cho ENTERPRISE plan, bao gồm kênh hỗ trợ ưu tiên, cam kết thời gian phản hồi (SLA), và tích hợp ticketing system trong sản phẩm.

### 1.2 Bối cảnh
| Plan       | Support           |
|------------|-------------------|
| FREE       | Community         |
| TEAM       | Email             |
| ENTERPRISE | Dedicated SLA     |

### 1.3 Mục tiêu
- Cung cấp kênh hỗ trợ ưu tiên cho ENTERPRISE customers
- Cam kết SLA response times theo severity
- Tích hợp support workflow trong sản phẩm (in-app support)
- Hỗ trợ escalation và dedicated account management

---

## 2. Yêu cầu chức năng

### FR-001: SLA Response Time Commitments
- **Mô tả**: Định nghĩa SLA targets theo severity level.
- **SLA Matrix**:

| Severity    | Mô tả                                      | Response Time | Resolution Time |
|-------------|---------------------------------------------|---------------|-----------------|
| **P0/SEV1** | Production down, data loss risk             | ≤ 1 hour      | ≤ 4 hours       |
| **P1/SEV2** | Major feature broken, workaround exists      | ≤ 4 hours     | ≤ 1 business day|
| **P2/SEV3** | Minor feature issue, non-critical            | ≤ 8 hours     | ≤ 3 business days|
| **P3/SEV4** | Enhancement request, documentation           | ≤ 1 biz day   | Best effort     |

- **Acceptance Criteria**:
  - AC-1: SLA timer bắt đầu từ lúc ticket được tạo
  - AC-2: SLA breach notification cho cả customer và internal team
  - AC-3: SLA pause khi chờ customer response (status: Waiting on Customer)

### FR-002: Support Channels
- **Mô tả**: Cung cấp multiple support channels cho ENTERPRISE.
- **Channels**:

| Channel              | FREE  | TEAM  | ENTERPRISE |
|----------------------|-------|-------|------------|
| Community Forum      | ✅    | ✅    | ✅         |
| Documentation        | ✅    | ✅    | ✅         |
| Email Support        | ❌    | ✅    | ✅         |
| In-App Chat          | ❌    | ❌    | ✅         |
| Slack/Teams Channel  | ❌    | ❌    | ✅         |
| Phone Escalation     | ❌    | ❌    | ✅         |
| Dedicated CSM        | ❌    | ❌    | ✅         |

- **Acceptance Criteria**:
  - AC-1: In-app chat widget visible cho ENTERPRISE users
  - AC-2: Slack integration cho real-time support communication
  - AC-3: Dedicated Customer Success Manager assigned per account

### FR-003: In-App Support Ticket System
- **Mô tả**: Tích hợp support ticket creation và tracking trong sản phẩm.
- **Features**:
  - Tạo support ticket trực tiếp từ UI
  - Attach diagnostics (version, environment, error logs)
  - Track ticket status và SLA timeline
  - View ticket history
- **Acceptance Criteria**:
  - AC-1: Support ticket form accessible từ Help menu
  - AC-2: Auto-attach system diagnostics (version, plan, instance count, last errors)
  - AC-3: Ticket status updates visible in-app
  - AC-4: Email notification cho ticket updates

### FR-004: Diagnostic Data Collection
- **Mô tả**: Tự động collect diagnostic data khi tạo support ticket.
- **Collected Data**:
  ```json
  {
    "system": {
      "bytebase_version": "v2.x.x",
      "go_version": "1.26",
      "os": "linux/amd64",
      "deployment": "docker | kubernetes | binary",
      "database_version": "PostgreSQL 16.x"
    },
    "workspace": {
      "plan": "ENTERPRISE",
      "instance_count": 25,
      "user_count": 150,
      "project_count": 12
    },
    "recent_errors": [
      {"timestamp": "...", "error": "...", "stack": "..."}
    ]
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Diagnostic collection là opt-in (user phải confirm)
  - AC-2: Sensitive data (passwords, tokens, DB credentials) KHÔNG bao giờ collected
  - AC-3: User có thể review diagnostic data trước khi submit

### FR-005: Support Dashboard
- **Mô tả**: Dashboard cho workspace admin để theo dõi support activities.
- **Features**:
  - Open/closed ticket count
  - SLA compliance rate
  - Average response/resolution time
  - Ticket history và trend
- **Acceptance Criteria**:
  - AC-1: Dashboard accessible cho workspace admins
  - AC-2: Filter by time period, severity, status
  - AC-3: Export support metrics (CSV)

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                           | Thay đổi                                       |
|------------------------------|----------------------------------------|-------------------------------------------------|
| Support Service (gRPC)       | `backend/api/v1/support_service.go`    | CRUD support tickets                            |
| Feature Gate                 | `enterprise/feature.go`               | Define `FeatureDedicatedSupport`                |
| Webhook                      | `backend/component/webhook/`           | SLA breach notifications                        |
| Diagnostic Collector         | `backend/component/diagnostic/`        | System info collection                          |
| Settings Store               | `backend/store/setting.go`             | Support channel configuration                   |

### 3.2 Frontend Changes

| Component             | File                                         | Thay đổi                                  |
|-----------------------|----------------------------------------------|--------------------------------------------|
| Support Widget        | `frontend/src/components/SupportWidget.vue`   | In-app chat/ticket widget                 |
| Support Page          | `frontend/src/views/Support.vue`              | Ticket list và detail                      |
| Help Menu             | `frontend/src/components/HelpMenu.vue`        | Link to support features                   |
| Support Dashboard     | `frontend/src/views/SupportDashboard.vue`     | Metrics và analytics                       |
| Diagnostic Dialog     | `frontend/src/components/DiagnosticDialog.vue` | Review/confirm diagnostic data            |

### 3.3 Database Changes

```sql
-- Support tickets table
CREATE TABLE support_ticket (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    creator_uid BIGINT NOT NULL REFERENCES principal(id),
    severity TEXT NOT NULL CHECK (severity IN ('SEV1', 'SEV2', 'SEV3', 'SEV4')),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'IN_PROGRESS', 'WAITING_CUSTOMER', 'RESOLVED', 'CLOSED')),
    sla_response_deadline TIMESTAMPTZ,
    sla_resolution_deadline TIMESTAMPTZ,
    first_responded_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    diagnostic_data JSONB,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_support_ticket_workspace ON support_ticket (workspace, status);
CREATE INDEX idx_support_ticket_creator ON support_ticket (creator_uid);
CREATE INDEX idx_support_ticket_sla ON support_ticket (sla_response_deadline) WHERE status = 'OPEN';
```

### 3.4 Proto Changes

```protobuf
service SupportService {
  rpc CreateTicket(CreateTicketRequest) returns (Ticket);
  rpc GetTicket(GetTicketRequest) returns (Ticket);
  rpc ListTickets(ListTicketsRequest) returns (ListTicketsResponse);
  rpc UpdateTicket(UpdateTicketRequest) returns (Ticket);
  rpc GetSupportMetrics(GetSupportMetricsRequest) returns (SupportMetrics);
  rpc CollectDiagnostics(CollectDiagnosticsRequest) returns (DiagnosticReport);
}
```

---

## 4. Phụ thuộc

| Dependency            | Mô tả                                                      |
|-----------------------|--------------------------------------------------------------|
| License Service       | Xác định plan để gate support features                       |
| Webhook Manager       | SLA breach notifications                                     |
| IM Integration        | Slack/Teams channel cho support communication                |
| External Ticketing    | Optional integration với Zendesk/Jira Service Management     |

---

## 5. Test Cases

| Test ID    | Mô tả                                                       | Expected Result                       |
|------------|---------------------------------------------------------------|---------------------------------------|
| TC-001     | ENTERPRISE user tạo support ticket                           | Ticket created với SLA deadlines      |
| TC-002     | FREE user thấy support widget                                | Widget hidden hoặc redirect community |
| TC-003     | SEV1 ticket SLA breach (> 1h no response)                    | Notification sent to internal team    |
| TC-004     | Diagnostic data collection                                    | System info collected, sensitive stripped |
| TC-005     | Ticket status transition: OPEN → IN_PROGRESS → RESOLVED      | Valid transitions, timestamps logged  |
| TC-006     | SLA pause khi status = WAITING_CUSTOMER                       | SLA timer paused                      |
| TC-007     | Support dashboard metrics accuracy                            | Correct counts and averages           |
| TC-008     | Export support metrics                                        | CSV download with correct data        |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Support ticket CRUD + DB migration   | Sprint 1       |
| Phase 2 | In-app support widget                | Sprint 2       |
| Phase 3 | SLA tracking + notifications         | Sprint 2       |
| Phase 4 | Diagnostic data collection           | Sprint 3       |
| Phase 5 | Support dashboard + metrics          | Sprint 3       |
| Phase 6 | External integration (Slack/Zendesk) | Sprint 4       |
