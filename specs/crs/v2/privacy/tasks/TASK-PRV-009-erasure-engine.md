# TASK-PRV-009 — Erasure Engine + DSR SLA Runner + Dashboard

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-009                               |
| **Source**       | SOL-PRV-003 Phase 3–5 (CR-PRV-003)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 3–4                                 |

---

## Mô tả

Xây dựng Erasure Engine cho cross-database data deletion, DSR SLA monitoring runner, và DSR Dashboard.

## Scope

1. **L5 — `component/privacy/erasure.go`**: ErasureEngine — `Execute()` method, 3 erasure methods: Hard Delete, Soft Delete, Anonymize
2. **Cross-database execution**: Dùng DB Driver (L7) để execute DELETE/UPDATE trên target databases
3. **Verification scan**: Post-erasure scan confirms no raw PII remains
4. **L6 — `runner/dsr/sla_runner.go`**: DSR SLA monitoring — periodic check (1h), 7-day warning, SLA breach alert
5. **L1 — `DSRDashboard.tsx`**: DSR management UI — list, status, SLA countdown, impact assessment
6. **Cascade erasure**: Handle FK dependencies across related tables/databases

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/privacy/erasure.go` | NEW — Erasure engine |
| `backend/runner/dsr/sla_runner.go` | NEW — SLA monitoring |
| `frontend/src/react/pages/privacy/DSRDashboard.tsx` | NEW — Dashboard UI |

## Acceptance Criteria

- [ ] 3 erasure methods: Hard Delete, Soft Delete, Anonymize
- [ ] Cascade erasure across related tables/databases
- [ ] Verification report proving data deletion
- [ ] Post-erasure PII scan verification
- [ ] Exception handling for legal hold obligations
- [ ] SLA runner: 7-day warning, breach alert via webhook
- [ ] DSR Dashboard: list, status tracking, SLA countdown

## Dependencies

- TASK-PRV-008 (DSR Service)
- TASK-PRV-005 (Anonymization engine for erasure-by-anonymization)
- TASK-PRV-010 (Legal hold — soft dependency)

## Definition of Done

- Erasure tested across 3+ DB engines
- SLA monitoring verified
- Dashboard functional with real data
