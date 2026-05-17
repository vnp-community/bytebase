# Change Request: Data Export Access Control

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-006                                               |
| **Feature ID**     | SQL-15, SQL-09 (extends), UR-S02                         |
| **Title**          | Data Export Access Control                               |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-005 (Restrict Copying), CR-PRV-002               |

---

## 1. Tổng quan

### 1.1 Mô tả
Mở rộng khả năng kiểm soát xuất dữ liệu (CR-ENT-005 Restrict Copying) với **privacy-aware export controls** — đảm bảo dữ liệu PII/sensitive không bị rò rỉ qua export, copy, hoặc data download.

### 1.2 Bối cảnh từ PRD/URD
- **PRD SQL-15**: Restrict Copying Data — chỉ block copy/paste, chưa cover export flows
- **PRD SQL-09**: Data Export — export CSV/Excel/JSON không kiểm soát PII content
- **URD UR-S02**: Data masking → nhưng masking bypass khi export raw data

### 1.3 Mục tiêu
- Export approval workflow cho sensitive data
- Automatic PII detection + masking/anonymization trước khi export
- Export volume và frequency limits
- DLP (Data Loss Prevention) rules cho export channels

---

## 2. Yêu cầu chức năng

### FR-001: Export Approval Workflow
- **Mô tả**: Export dữ liệu chứa PII/sensitive cần approval.
- **Workflow**:
  ```
  User requests export → System scans for PII/sensitive columns
    → [No PII] Auto-approve → Export
    → [Has PII] Approval required → DBA/Admin reviews
      → [Approved] Export with masking applied
      → [Rejected] User notified with reason
  ```
- **Acceptance Criteria**:
  - AC-1: Auto-approve cho non-sensitive data exports
  - AC-2: Configurable approval rules per classification level
  - AC-3: Export request tracked in audit log
  - AC-4: Expiry time cho approved exports (download link expires)

### FR-002: Export-Time Data Protection
- **Mô tả**: Apply masking/anonymization tại thời điểm export.
- **Protection Modes**:

| Mode             | Mô tả                                        | Use Case              |
|------------------|-----------------------------------------------|----------------------|
| **Masked**       | Apply data masking rules to export            | Analyst needs data   |
| **Anonymized**   | Apply anonymization (CR-PRV-002)              | Dev/staging use      |
| **Aggregated**   | Only export aggregated/summary data           | Reporting            |
| **Full**         | Raw data (requires elevated approval)         | Authorized access    |

- **Acceptance Criteria**:
  - AC-1: Default protection mode configurable per project
  - AC-2: Protection mode selection per export request
  - AC-3: Exported files include privacy metadata header

### FR-003: Export Rate Limiting & DLP
- **Mô tả**: Prevent data exfiltration qua export abuse.
- **Controls**:
  - Max export rows per request (default: 10,000)
  - Max exports per user per day (default: 10)
  - Max total export volume per day (default: 100MB)
  - Alert on anomalous export patterns
- **Acceptance Criteria**:
  - AC-1: Rate limits configurable per role/project
  - AC-2: Real-time alerting on limit breaches
  - AC-3: Export blocked when limits exceeded

### FR-004: Export Audit Trail
- **Mô tả**: Complete audit trail cho tất cả data exports.
- **Logged Information**: Who, when, what query, which columns, row count, file format, protection mode, download IP, approval chain
- **Acceptance Criteria**:
  - AC-1: Every export logged regardless of size
  - AC-2: Export files tagged with trace ID for forensic correlation
  - AC-3: Export reports for compliance review

---

## 3. Yêu cầu kỹ thuật

| Component               | File/Package                                | Thay đổi                          |
|--------------------------|---------------------------------------------|-----------------------------------|
| Export Policy Service    | `backend/component/export/policy.go`        | Export approval + DLP rules       |
| Export Masking Pipeline  | `backend/component/export/privacy.go`       | Apply masking on export           |
| Rate Limiter             | `backend/component/export/rate_limiter.go`  | Export rate limiting              |
| Export Audit             | `backend/component/export/audit.go`         | Export audit trail                |
| Feature Gate             | `enterprise/feature.go`                     | `FeatureExportDLP`                |

---

## 4. Rollout Plan

| Phase   | Mô tả                                   | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Export approval workflow                  | Sprint 1       |
| Phase 2 | Export-time masking integration            | Sprint 2       |
| Phase 3 | Rate limiting + DLP rules                 | Sprint 2       |
| Phase 4 | Export audit trail + compliance reports    | Sprint 3       |
