# TASK-PRV-017 — Post-Sync Verification + Data Flow Monitor + Dashboard

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-017                               |
| **Source**       | SOL-PRV-007 Phase 3–4 (CR-PRV-007)        |
| **Status**       | Pending                                    |
| **Priority**     | P2                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Post-sync PII verification scan, cross-environment data flow monitoring runner, và Data Isolation Dashboard.

## Scope

1. **Post-sync verification**: Scan target database sau sync — confirm zero raw PII (confidence > 0.8)
2. **L6 — `runner/monitor/data_flow.go`**: DataFlowMonitor — periodic (5 min), detect unauthorized cross-env data access via audit logs
3. **Alert**: Webhook alert within 5 minutes of policy violation
4. **L1 — `DataIsolation.tsx`**: Dashboard — active data flows visualization (prod ↔ staging ↔ dev), isolation policy management, weekly compliance report

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/sync_pipeline.go` | MODIFY — Add verification |
| `backend/runner/monitor/data_flow.go` | NEW — Data flow monitor |
| `frontend/src/react/pages/privacy/DataIsolation.tsx` | NEW — Dashboard |

## Acceptance Criteria

- [ ] Post-sync verification: zero raw PII in target (confidence > 0.8)
- [ ] Verification failure: rollback + alert
- [ ] Data flow monitor: detect cross-env access within 5 minutes
- [ ] Alert on unauthorized data flow via webhook
- [ ] Dashboard: real-time data flow visualization
- [ ] Weekly data isolation compliance report

## Dependencies

- TASK-PRV-016 (Isolation policy engine)
- TASK-PRV-001 (PII Scanner for verification)

## Definition of Done

- Verification scan tested with known PII data
- Data flow monitor alert latency < 5 minutes
- Dashboard functional
