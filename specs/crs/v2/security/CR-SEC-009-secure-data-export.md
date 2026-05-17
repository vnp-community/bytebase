# Change Request: Secure Data Export & Transfer Controls

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-009                                               |
| **Feature ID**     | SQL-15, SEC-15                                           |
| **Title**          | Secure Data Export & Transfer Controls                   |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Kiểm soát toàn diện việc export và transfer dữ liệu từ Bytebase: export approval workflow, row/size limits, masking enforcement trong exports, encrypted export files, và data loss prevention (DLP) integration.

### 1.2 Bối cảnh
Bytebase hỗ trợ Data Export (SQL-09), Restrict Copying (SQL-15), và Data Masking (SEC-15). CR này bổ sung controls cho export pipeline để ngăn data exfiltration.

---

## 2. Yêu cầu chức năng

### FR-001: Export Approval Workflow
- **Acceptance Criteria**:
  - AC-1: Export requests >N rows require approval (configurable threshold)
  - AC-2: Approval workflow per environment (prod requires approval, dev auto-approve)
  - AC-3: Export request includes justification field
  - AC-4: Time-limited export approval (expires after 24h)
  - AC-5: All exports logged with requester, approver, query, row count

### FR-002: Export Content Controls
- **Acceptance Criteria**:
  - AC-1: Maximum export row limit (configurable, default 10,000)
  - AC-2: Maximum export file size limit (configurable, default 100MB)
  - AC-3: Masking rules enforced in exported data
  - AC-4: Sensitive columns excluded from export by default
  - AC-5: Export format restrictions (admin can disable CSV/JSON/SQL dump)

### FR-003: Encrypted Export
- **Acceptance Criteria**:
  - AC-1: Option to encrypt exported files (AES-256)
  - AC-2: Password-protected ZIP/archive option
  - AC-3: Expiring download links (configurable TTL)
  - AC-4: Download count limit per export

### FR-004: DLP Integration
- **Acceptance Criteria**:
  - AC-1: Content scanning before export (PII, credit card, SSN patterns)
  - AC-2: Auto-block exports containing classified data
  - AC-3: DLP violation alerts to security team
  - AC-4: Integration points for external DLP tools (webhook/API)

### FR-005: Clipboard & Screenshot Protection
- Enhancement of SQL-15 (Restrict Copying Data)
- **Acceptance Criteria**:
  - AC-1: Disable clipboard copy for masked data
  - AC-2: Disable right-click context menu on sensitive results
  - AC-3: Watermark overlay on query results (link CR-ENT-021)
  - AC-4: Screenshot detection notification (best-effort)

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Export Service               | `backend/component/export/`                 | Approval workflow, content controls         |
| DLP Scanner (new)            | `backend/component/dlp/`                    | Content scanning, pattern matching          |
| Export Policy Service        | `backend/api/v1/org_policy_service.go`      | Export policy CRUD                          |
| SQL Service                  | `backend/api/v1/sql_service.go`             | Export request interception                 |
| Export UI                    | `frontend/src/components/ExportDialog.vue`  | Approval UI, encryption options             |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Export >10K rows from production                     | Approval required                |
| TC-002  | Export masked column                                 | Data remains masked in export    |
| TC-003  | Export with PII content                              | DLP blocks or warns              |
| TC-004  | Encrypted export download                            | Requires password to open        |
| TC-005  | Download link after TTL expired                      | Link invalid                     |
| TC-006  | Copy masked data from SQL Editor                     | Clipboard blocked                |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Export approval workflow             | Sprint 1       |
| Phase 2 | Content controls + size limits       | Sprint 2       |
| Phase 3 | Encrypted export + expiring links    | Sprint 3       |
| Phase 4 | DLP integration                      | Sprint 4       |
