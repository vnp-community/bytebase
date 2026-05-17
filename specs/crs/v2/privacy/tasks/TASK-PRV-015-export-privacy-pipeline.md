# TASK-PRV-015 — Export Privacy Pipeline + Audit Trail

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-015                               |
| **Source**       | SOL-PRV-006 Phase 2, 4–5 (CR-PRV-006)     |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 2–3                                 |

---

## Mô tả

Integrate masking/anonymization vào export pipeline và xây dựng complete export audit trail.

## Scope

1. **L5 — `component/export/privacy.go`**: Export privacy pipeline — apply masking (via MaskingEvaluator) hoặc anonymization (via AnonymizationEngine) tại export time
2. **Protection modes**: Masked, Anonymized, Aggregated, Full (requires elevated approval)
3. **L4 — `sql_service.go`** (modify): Protection mode selection per export request
4. **Export audit trail**: Complete log — who, when, query, columns, row count, format, protection mode, download IP, approval chain
5. **Export approval workflow**: Pending export requests, approval/rejection, expiry time cho download links
6. **Trace ID**: Each export tagged with unique trace ID for forensic correlation

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/component/export/privacy.go` | NEW — Privacy pipeline |
| `backend/api/v1/sql_service.go` | MODIFY — Protection modes |
| `backend/store/export_policy.go` | MODIFY — Export audit CRUD |

## Acceptance Criteria

- [ ] 4 protection modes: Masked, Anonymized, Aggregated, Full
- [ ] Default protection mode configurable per project
- [ ] Exported files include privacy metadata header
- [ ] Every export logged regardless of size
- [ ] Export files tagged with trace ID
- [ ] Approval workflow for sensitive exports
- [ ] Approved export: single-use + expiry time
- [ ] Export reports for compliance review

## Dependencies

- TASK-PRV-014 (Export policy engine)
- TASK-PRV-005 (Anonymization engine — for anonymized mode)
- TASK-ENT-015 (Data Masking — for masked mode)

## Definition of Done

- All 4 protection modes tested
- Audit trail complete and verifiable
- Approval workflow end-to-end tested
