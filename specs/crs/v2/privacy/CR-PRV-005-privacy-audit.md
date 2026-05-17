# Change Request: Privacy-Preserving Query Audit

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-005                                               |
| **Feature ID**     | SEC-07, SEC-10 (extends), UR-S03                         |
| **Title**          | Privacy-Preserving Query Audit                           |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-003 (Audit Log Full), CR-PRV-008                 |

---

## 1. Tổng quan

### 1.1 Mô tả
Tăng cường Audit Log (CR-ENT-003) với khả năng **privacy-preserving** — đảm bảo audit logs ghi nhận đầy đủ hoạt động nhưng **không lưu trữ PII/sensitive data** trong log entries. Audit log phải đủ chi tiết cho forensics nhưng không trở thành nguồn rò rỉ dữ liệu mới.

### 1.2 Bối cảnh từ PRD/URD
- **PRD SEC-07/10**: Audit Log lưu mọi API call — có thể chứa PII trong query text, parameters
- **URD UR-S03**: Full audit cho mọi API call — risk: query chứa WHERE email='john@example.com'
- **URD UR-D05**: DBA xem audit log — cần xem activity nhưng không cần thấy actual PII

### 1.3 Mục tiêu
- PII sanitization trong audit log entries
- Query text redaction (mask literals in SQL queries)
- Configurable audit detail levels per data classification
- Audit log access control (who can see what level of detail)

---

## 2. Yêu cầu chức năng

### FR-001: Query Text Redaction
- **Mô tả**: Tự động redact PII literals trong SQL queries trước khi lưu vào audit log.
- **Redaction Rules**:
  ```sql
  -- Original query (logged by user):
  SELECT * FROM users WHERE email = 'john@example.com' AND ssn = '123-45-6789';
  
  -- Redacted query (stored in audit log):
  SELECT * FROM users WHERE email = '[REDACTED]' AND ssn = '[REDACTED]';
  ```
- **Acceptance Criteria**:
  - AC-1: String literals trong WHERE/SET clauses auto-redacted
  - AC-2: Table/column names preserved (không redact)
  - AC-3: Redaction configurable: OFF, LITERALS_ONLY, FULL
  - AC-4: Original query hash stored for forensic correlation

### FR-002: Audit Detail Levels
- **Mô tả**: Configurable detail levels cho audit entries.
- **Levels**:

| Level        | Logged Information                                  | PII Exposure  |
|-------------|-----------------------------------------------------|---------------|
| **MINIMAL** | Action type, user, timestamp, resource ID           | None          |
| **STANDARD**| + Query structure (redacted), affected row count    | None          |
| **DETAILED**| + Full query (redacted), execution plan             | Minimal       |
| **FORENSIC**| + Original query (encrypted at rest)                | Encrypted     |

- **Acceptance Criteria**:
  - AC-1: Default level: STANDARD
  - AC-2: FORENSIC level requires explicit admin approval
  - AC-3: FORENSIC data encrypted with separate key (break-glass access)

### FR-003: Audit Log Access Control
- **Mô tả**: Phân quyền xem audit log theo sensitivity level.
- **Access Tiers**:
  - **Viewer**: MINIMAL + STANDARD entries
  - **Auditor**: + DETAILED entries
  - **Forensic Analyst**: + FORENSIC entries (with break-glass)
- **Acceptance Criteria**:
  - AC-1: Role-based audit log access
  - AC-2: Break-glass access for FORENSIC requires justification + approval
  - AC-3: Access to audit logs itself audited (meta-audit)

### FR-004: Sensitive Parameter Filtering
- **Mô tả**: Filter sensitive parameters từ API request/response trước khi audit.
- **Filtered Fields**: Password, token, secret, API key, connection string credentials
- **Acceptance Criteria**:
  - AC-1: Sensitive HTTP headers filtered (Authorization, Cookie)
  - AC-2: Connection string passwords masked
  - AC-3: Configurable filter patterns

---

## 3. Yêu cầu kỹ thuật

| Component                | File/Package                              | Thay đổi                           |
|--------------------------|-------------------------------------------|-------------------------------------|
| Query Redactor           | `backend/component/privacy/redactor.go`   | SQL literal redaction engine        |
| Audit Interceptor        | `backend/api/v1/audit.go`                 | Privacy-aware audit logging         |
| Parameter Filter         | `backend/component/privacy/param_filter.go`| Sensitive param filtering          |
| Audit Access Control     | `backend/api/v1/audit_log_service.go`     | Tiered access enforcement          |
| Feature Gate             | `enterprise/feature.go`                   | `FeaturePrivacyAudit`              |

---

## 4. Rollout Plan

| Phase   | Mô tả                                   | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Query text redaction engine               | Sprint 1       |
| Phase 2 | Audit detail levels + access control      | Sprint 2       |
| Phase 3 | Sensitive parameter filtering             | Sprint 2       |
| Phase 4 | FORENSIC level encryption + break-glass   | Sprint 3       |
