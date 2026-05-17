# Change Request: Compliance Reporting Framework

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-012                                               |
| **Feature ID**     | SEC-10, SEC-16                                           |
| **Title**          | Compliance Reporting Framework                           |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Framework báo cáo tuân thủ tự động cho các tiêu chuẩn bảo mật: SOC 2 Type II, ISO 27001, GDPR, HIPAA, PCI DSS. Tự động thu thập evidence, đánh giá compliance status, và generate audit-ready reports.

---

## 2. Yêu cầu chức năng

### FR-001: Compliance Framework Templates
- **Supported Frameworks**:

| Framework    | Focus Areas                                              |
|-------------|----------------------------------------------------------|
| SOC 2       | Access control, audit logging, encryption, availability   |
| ISO 27001   | Information security management, risk assessment          |
| GDPR        | Data protection, consent, right to erasure, DPO           |
| HIPAA       | PHI protection, access controls, audit trails             |
| PCI DSS     | Cardholder data protection, encryption, access control    |

- **Acceptance Criteria**:
  - AC-1: Pre-built control mappings per framework
  - AC-2: Custom framework support (define own controls)
  - AC-3: Control-to-feature mapping (which Bytebase features satisfy which controls)

### FR-002: Automated Evidence Collection
- **Evidence Sources**:
  - Audit logs → access control evidence
  - Masking policies → data protection evidence
  - Encryption status → encryption at rest evidence
  - Password policies → authentication controls evidence
  - SSO configuration → identity management evidence
  - Role assignments → authorization evidence
- **Acceptance Criteria**:
  - AC-1: Automatic evidence snapshot on schedule
  - AC-2: Evidence linked to specific compliance controls
  - AC-3: Evidence versioning and retention
  - AC-4: Gap identification — missing evidence highlighted

### FR-003: Compliance Dashboard
- **Acceptance Criteria**:
  - AC-1: Overall compliance score per framework
  - AC-2: Control-by-control status (compliant/non-compliant/partial)
  - AC-3: Trend tracking over time
  - AC-4: Action items for non-compliant controls
  - AC-5: Exportable PDF/HTML reports for external auditors

### FR-004: Continuous Compliance Monitoring
- **Acceptance Criteria**:
  - AC-1: Real-time compliance status updates
  - AC-2: Alert on compliance regression (e.g., masking policy removed)
  - AC-3: Scheduled compliance assessment (daily/weekly)
  - AC-4: Integration with ticketing systems for remediation tracking

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Compliance Engine (new)      | `backend/component/compliance/`             | Framework engine, evidence collection       |
| Compliance Service (new)     | `backend/api/v1/compliance_service.go`      | Compliance API endpoints                    |
| Report Generator (new)       | `backend/component/compliance/report/`      | PDF/HTML report generation                  |
| Compliance Dashboard         | `frontend/src/views/ComplianceDashboard.vue`| Compliance status UI                        |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | SOC 2 compliance assessment                          | Score with control-level details |
| TC-002  | Masking policy removed → compliance check            | Compliance regression alert      |
| TC-003  | Generate PDF report for auditor                      | Complete audit-ready report      |
| TC-004  | Evidence snapshot captured                           | Evidence linked to controls      |
| TC-005  | Custom framework created                             | Framework with custom controls   |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Framework templates + control mapping| Sprint 1       |
| Phase 2 | Evidence collection engine           | Sprint 2       |
| Phase 3 | Compliance dashboard                 | Sprint 3       |
| Phase 4 | Report generation + monitoring       | Sprint 4       |
