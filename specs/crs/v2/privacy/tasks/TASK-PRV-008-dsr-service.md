# TASK-PRV-008 — DSR Service + Workflow Engine

| Field            | Value                                      |
|------------------|----------------------------------------    |
| **Task ID**      | TASK-PRV-008                               |
| **Source**       | SOL-PRV-003 Phase 1–2 (CR-PRV-003)        |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Xây dựng Data Subject Request (DSR) service và consent management — workflow engine cho GDPR/PDPA compliance.

## Scope

1. **L4 — `api/v1/dsr_service.go`**: DSR CRUD — `CreateDSR`, `GetDSR`, `ListDSRs`, `ProcessDSR`, `VerifyDSR`
2. **DSR types**: ACCESS, RECTIFICATION, ERASURE, RESTRICTION, PORTABILITY, OBJECTION
3. **Workflow**: DSR Submitted → Identity Verified → Impact Assessment → Approval → Execute → Verify → Close
4. **L4 — `api/v1/consent_service.go`**: Consent records CRUD — per user, per purpose, with expiry
5. **L8 — Migration**: Tạo bảng `dsr` + `consent_record` + indexes
6. **L8 — `store/dsr.go`, `store/consent.go`**: Store CRUD
7. **Approval integration**: Reuse existing Approval Runner (L6) via Bus
8. **SLA**: 30-day deadline per DSR type (GDPR/PDPA)

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/dsr_service.go` | NEW — DSR service |
| `backend/api/v1/consent_service.go` | NEW — Consent management |
| `backend/store/dsr.go` | NEW — DSR store |
| `backend/store/consent.go` | NEW — Consent store |
| `backend/store/migration/` | NEW — DDL migration |
| `backend/enterprise/feature.go` | ADD — `FeatureDataSubjectRights` |

## Acceptance Criteria

- [ ] DSR CRUD: all 6 request types supported
- [ ] Identity verification step mandatory
- [ ] Auto-discovery of data subject's data via PII Scanner
- [ ] Approval workflow via existing Approval Runner
- [ ] SLA countdown: 30-day deadline with tracking
- [ ] Consent records: per purpose, granular, with expiry
- [ ] Consent withdrawal revokes access within 24h
- [ ] Consent history fully audited

## Dependencies

- TASK-PRV-001 (PII Scanner for data discovery)
- TASK-ENT-010 (Approval Workflow)

## Definition of Done

- DSR workflow end-to-end tested
- Consent CRUD functional
- SLA tracking verified
