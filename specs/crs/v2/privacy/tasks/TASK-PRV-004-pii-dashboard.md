# TASK-PRV-004 — PII Inventory Dashboard + Compliance Mapping

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-004                               |
| **Source**       | SOL-PRV-001 Phase 4–6 (CR-PRV-001)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 3                                   |

---

## Mô tả

Xây dựng PII Inventory dashboard (React) và compliance mapping reports.

## Scope

1. **L1 — `PIIInventory.tsx`**: Dashboard — coverage overview, PII heatmap (database → table → column drill-down), unscanned report, risk score
2. **L1 — `PIIHeatmap.tsx`**: Visual heatmap PII density
3. **Export**: Inventory export (CSV, JSON, PDF)
4. **Compliance mapping**: GDPR Article 30, PDPA, HIPAA PHI tracking, PCI-DSS — per-regulation report template + gap analysis
5. **Multi-engine testing**: Validate PII scanner trên 22+ DB engines, optimize performance

## Files cần thay đổi

| File | Action |
|------|--------|
| `frontend/src/react/pages/privacy/PIIInventory.tsx` | NEW — Dashboard |
| `frontend/src/react/pages/privacy/PIIHeatmap.tsx` | NEW — Heatmap |

## Acceptance Criteria

- [ ] Real-time dashboard: drill-down database → table → column
- [ ] Export inventory CSV, JSON, PDF
- [ ] Filter by PII category, classification level, engine
- [ ] Compliance report template cho GDPR, PDPA, HIPAA, PCI-DSS
- [ ] Gap analysis: unscanned databases, unconfirmed classifications

## Dependencies

- TASK-PRV-001, TASK-PRV-002, TASK-PRV-003

## Definition of Done

- Dashboard functional with real scan data
- Export formats validated
- Compliance reports reviewed
