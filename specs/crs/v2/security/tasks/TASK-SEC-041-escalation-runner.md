# TASK-SEC-041 — Escalation Runner + SLA Monitoring

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-041                               |
| **Source**       | SOL-SEC-018 §3.3                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Implement Escalation Runner (L6) consuming IncidentChan, SLA monitoring, auto-escalation via existing Webhook Manager.

## Scope

1. **Runner**: `runner/incident/escalation.go` — 1min ticker for SLA check + IncidentChan consumer
2. **SLA config**: SEV1=15min, SEV2=1h, SEV3=4h, SEV4=24h — configurable
3. **Escalation**: `webhookManager.Send()` — Slack, DingTalk, Feishu, Teams (existing L5 component)
4. **Multi-level escalation**: Level 1 → on-call engineer, Level 2 → security lead, Level 3 → CISO
5. **Initial notification**: On incident creation → immediate notification via IncidentChan

## Acceptance Criteria

- [ ] SLA breach triggers escalation
- [ ] Initial notification immediate
- [ ] Multi-channel delivery (Slack, etc.)
- [ ] SLA config customizable

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/runner/incident/escalation.go` | New file |
| `backend/server/server.go` | Bootstrap |

## Definition of Done

- SLA monitoring tested with mock incidents
