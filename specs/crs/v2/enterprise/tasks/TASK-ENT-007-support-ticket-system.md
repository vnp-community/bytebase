# TASK-ENT-007 — Support Ticket System & SLA Tracking

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-007                               |
| **Source**       | SOL-ENT-004 (CR-ENT-004)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Xây dựng in-app support ticket system với SLA tracking, diagnostic collection, và multi-channel notification.

## Scope

### Phase 1 — Sprint 1: Ticket CRUD + DB Migration
1. **Schema Migration**: Create `support_ticket` table với severity (SEV1-4), status flow, SLA deadlines
2. **L4 — SupportService (NEW)**: `support_service.go` — gRPC ticket CRUD
3. **L8 — Store**: `store/support_ticket.go` — ticket persistence
4. **L9 — Feature Gate**: `FeatureDedicatedSupport`

### Phase 2 — Sprint 2: Widget + SLA
5. **L1 — SupportWidget.vue**: In-app support widget
6. **L6 — SLA Checker Runner (NEW)**: `runner/sla/checker.go` — check breached tickets every 1 min
7. **SLA Matrix**: SEV1 (1h response/4h resolution), SEV2 (4h/1 BD), SEV3 (8h/3 BD), SEV4 (1 BD/best effort)
8. **Webhook Notifications**: SLA breach → Slack/Teams notification

### Phase 3 — Sprint 3: Diagnostics + Dashboard
9. **L5 — Diagnostic Collector (NEW)**: `component/diagnostic/collector.go` — system info collection (version, OS, deployment, counts, recent errors)
10. **Security**: Passwords, tokens, DB credentials NEVER collected. User opt-in + review before submit
11. **SupportDashboard.vue**: Dashboard + metrics view

### Phase 4 — Sprint 4: External Integration
12. Slack/Zendesk integration

## Acceptance Criteria

- [x] `support_ticket` table created via migration
- [x] Ticket CRUD API functional (create, update status, list, get)
- [x] SLA deadlines auto-calculated based on severity
- [x] SLAChecker runner detects breached tickets
- [x] Webhook notifications sent on SLA breach
- [x] Diagnostic collector gathers system info safely
- [x] No sensitive data in diagnostic reports
- [x] In-app widget functional
- [x] Support dashboard shows ticket metrics

## Dependencies

- CR-ENT-003 (Audit Log) — ticket actions logged

## Definition of Done

- [x] Full ticket lifecycle functional
- [x] SLA tracking accurate
- [x] Diagnostic collection secure
