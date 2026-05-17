# Change Request: User Consent & Data Subject Rights

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-003                                               |
| **Feature ID**     | UR-S03, UR-S14 (extends)                                 |
| **Title**          | User Consent & Data Subject Rights (GDPR/PDPA)           |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-003 (Audit Log), CR-PRV-001, CR-PRV-004          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai khung quản lý **quyền chủ thể dữ liệu (Data Subject Rights)** theo GDPR và PDPA, bao gồm: quyền truy cập, chỉnh sửa, xóa, hạn chế xử lý, và portability cho dữ liệu cá nhân được quản lý qua Bytebase.

### 1.2 Bối cảnh từ PRD/URD
- **URD UR-S03**: Audit log cho mọi API call — cần mở rộng để track data subject requests
- **URD UR-S14**: Email domain restriction — cần consent framework rộng hơn
- Bytebase quản lý metadata và query results chứa PII → cần DSR (Data Subject Request) workflow

### 1.3 Mục tiêu
- Data Subject Request (DSR) workflow: Access, Rectification, Erasure, Restriction, Portability
- Consent management cho data processing activities
- Automated data lineage tracking cho DSR fulfillment
- SLA tracking cho DSR response (GDPR: 30 days, PDPA: 30 days)

---

## 2. Yêu cầu chức năng

### FR-001: Data Subject Request (DSR) Workflow
- **Request Types**:

| Type              | GDPR Article | Mô tả                                    | SLA          |
|-------------------|-------------|---------------------------------------------|--------------|
| **Access**        | Art. 15     | Cung cấp bản sao dữ liệu cá nhân          | 30 days      |
| **Rectification** | Art. 16     | Sửa đổi dữ liệu không chính xác           | 30 days      |
| **Erasure**       | Art. 17     | Xóa dữ liệu (Right to be Forgotten)       | 30 days      |
| **Restriction**   | Art. 18     | Hạn chế xử lý dữ liệu                     | 30 days      |
| **Portability**   | Art. 20     | Export dữ liệu ở định dạng machine-readable | 30 days      |
| **Objection**     | Art. 21     | Phản đối xử lý dữ liệu                    | 30 days      |

- **Workflow**:
  ```
  DSR Submitted → Identity Verified → Impact Assessment
    → Approval (DBA/Admin) → Execution Plan Generated
      → Execute across affected databases → Verification
        → Response to Data Subject → Audit Log
  ```
- **Acceptance Criteria**:
  - AC-1: DSR portal cho data subjects hoặc privacy officers
  - AC-2: Identity verification step bắt buộc trước khi xử lý
  - AC-3: Automated discovery of data subject's data (via CR-PRV-001)
  - AC-4: SLA countdown timer với escalation alerts

### FR-002: Consent Management
- **Mô tả**: Quản lý consent records cho data processing.
- **Features**:
  - Consent record per user: purpose, scope, timestamp, expiry
  - Consent withdrawal mechanism
  - Processing activity register (GDPR Article 30)
  - Purpose limitation enforcement
- **Acceptance Criteria**:
  - AC-1: Consent phải explicit và granular (per purpose)
  - AC-2: Withdrawal revokes access within 24 hours
  - AC-3: Consent history fully audited

### FR-003: Right to Erasure Automation
- **Mô tả**: Automated data erasure across managed databases.
- **Erasure Methods**:
  - Hard delete: physical removal from database
  - Soft delete: logical deletion with retention period
  - Anonymization: replace PII with anonymous data (via CR-PRV-002)
- **Acceptance Criteria**:
  - AC-1: Cascade erasure across related tables/databases
  - AC-2: Verification report proving data deletion
  - AC-3: Backup data also anonymized/deleted
  - AC-4: Exception handling for legal hold obligations

---

## 3. Yêu cầu kỹ thuật

| Component              | File/Package                               | Thay đổi                          |
|------------------------|---------------------------------------------|-----------------------------------|
| DSR Service            | `backend/api/v1/dsr_service.go`            | DSR workflow management           |
| Consent Store          | `backend/store/consent.go`                 | Consent records CRUD              |
| Erasure Engine         | `backend/component/privacy/erasure.go`     | Cross-database data erasure       |
| DSR Dashboard          | `frontend/src/views/DSRDashboard.tsx`      | DSR management UI                 |
| Feature Gate           | `enterprise/feature.go`                    | `FeatureDataSubjectRights`        |

---

## 4. Rollout Plan

| Phase   | Mô tả                                   | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | DSR workflow engine + API                 | Sprint 1-2     |
| Phase 2 | Consent management                        | Sprint 2       |
| Phase 3 | Automated erasure engine                  | Sprint 3       |
| Phase 4 | DSR dashboard + SLA tracking              | Sprint 3-4     |
