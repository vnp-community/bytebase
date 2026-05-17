# Solution: CR-ENT-004 — Dedicated SLA Support

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-004                |
| **Solution**   | SOL-ENT-004               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Xây dựng **in-app support ticket system** tích hợp SLA tracking, diagnostic collection, và multi-channel notification. Thêm `SupportService` gRPC mới, bảng `support_ticket` trong metadata DB, và `SLAChecker` runner cho breach monitoring.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `support_service.go` (NEW) | CRUD tickets, diagnostic collection |
| **L5 — Component** | `component/diagnostic/` (NEW) | System info collector |
| **L5 — Component** | `component/webhook/` | SLA breach notifications |
| **L6 — Runner** | `runner/sla/` (NEW) | Background SLA deadline checker |
| **L8 — Store** | `store/support_ticket.go` (NEW) | Ticket persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureDedicatedSupport` gate |
| **L1 — Presentation** | `SupportWidget.vue`, `SupportDashboard.vue` | In-app widget + dashboard |

---

## 3. Chi tiết Implementation

### 3.1 Schema Migration

```sql
CREATE TABLE support_ticket (
    id BIGSERIAL PRIMARY KEY,
    workspace TEXT NOT NULL,
    creator_uid BIGINT NOT NULL REFERENCES principal(id),
    severity TEXT NOT NULL CHECK (severity IN ('SEV1', 'SEV2', 'SEV3', 'SEV4')),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN'
        CHECK (status IN ('OPEN', 'IN_PROGRESS', 'WAITING_CUSTOMER', 'RESOLVED', 'CLOSED')),
    sla_response_deadline TIMESTAMPTZ,
    sla_resolution_deadline TIMESTAMPTZ,
    first_responded_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    diagnostic_data JSONB,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_support_ticket_workspace ON support_ticket (workspace, status);
CREATE INDEX idx_support_ticket_sla ON support_ticket (sla_response_deadline) WHERE status = 'OPEN';
```

### 3.2 SLA Matrix (hardcoded, configurable via settings)

| Severity | Response Time | Resolution Time |
|----------|--------------|----------------|
| SEV1 | ≤ 1 hour | ≤ 4 hours |
| SEV2 | ≤ 4 hours | ≤ 1 business day |
| SEV3 | ≤ 8 hours | ≤ 3 business days |
| SEV4 | ≤ 1 business day | Best effort |

### 3.3 L6 — SLA Checker Runner

```go
// runner/sla/checker.go
func (c *SLAChecker) Run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            tickets, _ := c.store.FindBreachedTickets(ctx, time.Now())
            for _, t := range tickets {
                c.webhookManager.SendSLABreach(ctx, t)
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 3.4 Diagnostic Collector

```go
// component/diagnostic/collector.go
type DiagnosticReport struct {
    BytebaseVersion string
    GoVersion       string
    OS              string
    Deployment      string // docker | kubernetes | binary
    DatabaseVersion string
    InstanceCount   int
    UserCount       int
    ProjectCount    int
    RecentErrors    []ErrorEntry // last 10 errors, sanitized
}
```

**Security**: Passwords, tokens, DB credentials **NEVER** collected. User phải opt-in và review trước submit.

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-003 (Audit Log) | Support ticket actions logged |
| Webhook Manager | SLA breach notifications (Slack, Teams) |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Support ticket CRUD + DB migration | Sprint 1 |
| 2 | In-app support widget | Sprint 2 |
| 3 | SLA tracking + notifications | Sprint 2 |
| 4 | Diagnostic collection | Sprint 3 |
| 5 | Support dashboard + metrics | Sprint 3 |
| 6 | External integration (Slack/Zendesk) | Sprint 4 |
