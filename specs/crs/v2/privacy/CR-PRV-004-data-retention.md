# Change Request: Data Retention & Automated Purging

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-004                                               |
| **Feature ID**     | UR-D11, NF-SE01                                          |
| **Title**          | Data Retention & Automated Purging                       |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-PRV-003, CR-ENT-003                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống **Data Retention Policy** và **Automated Purging** để đảm bảo dữ liệu cá nhân không bị lưu trữ quá thời hạn cần thiết (GDPR Article 5(1)(e) — storage limitation). Áp dụng cho cả metadata trong Bytebase (query history, audit logs, session data) và hướng dẫn/enforcement cho managed databases.

### 1.2 Bối cảnh từ PRD/URD
- **URD UR-D11**: Clean up dữ liệu cũ tự động — hiện chỉ có basic cleaner runner
- **PRD SQL-10**: Query History — không có retention policy, lưu vô thời hạn
- **URD UR-S03**: Audit log cần retention policy theo compliance requirements

### 1.3 Mục tiêu
- Configurable retention policies per data type
- Automated purging scheduler (cron-based)
- Legal hold mechanism (suspend purging cho litigation)
- Retention compliance dashboard

---

## 2. Yêu cầu chức năng

### FR-001: Retention Policy Configuration
- **Policy Targets**:

| Data Type              | Default Retention | Configurable | Regulation        |
|------------------------|-------------------|-------------|-------------------|
| Query History          | 90 days           | ✅          | Internal          |
| Audit Logs             | 365 days          | ✅          | GDPR, SOC2        |
| Session Data           | 30 days           | ✅          | Internal          |
| Export Files           | 7 days            | ✅          | GDPR              |
| Backup Data            | 90 days           | ✅          | GDPR              |
| PII Scan Results       | 180 days          | ✅          | Internal          |
| Webhook Delivery Logs  | 30 days           | ✅          | Internal          |
| Change History         | Unlimited         | ✅          | Compliance        |

- **Acceptance Criteria**:
  - AC-1: Per-workspace và per-project retention settings
  - AC-2: Minimum retention enforced (cannot set below regulatory minimum)
  - AC-3: Policy changes audited
  - AC-4: Warning khi retention period sắp hết

### FR-002: Automated Purging Engine
- **Mô tả**: Scheduler tự động xóa dữ liệu hết hạn retention.
- **Features**:
  - Cron-based scheduler (configurable schedule)
  - Batch purging (avoid overloading database)
  - Dry-run mode: preview what will be purged
  - Purge verification report
- **Acceptance Criteria**:
  - AC-1: Purging chạy off-peak hours (configurable)
  - AC-2: Batch size configurable (default 1000 records/batch)
  - AC-3: Purge results logged to audit trail
  - AC-4: Alert on purge failures

### FR-003: Legal Hold
- **Mô tả**: Suspend purging cho specific data khi có litigation/investigation.
- **Features**:
  - Legal hold per user, per project, hoặc per database
  - Hold duration (indefinite hoặc time-limited)
  - Override retention policy during hold period
  - Hold release with approval workflow
- **Acceptance Criteria**:
  - AC-1: Legal hold prevents all automated purging for in-scope data
  - AC-2: Hold creation/release requires admin approval
  - AC-3: Hold status visible in retention dashboard

### FR-004: Retention Compliance Dashboard
- **Mô tả**: Dashboard giám sát tuân thủ retention policies.
- **Metrics**: Data volumes by retention status, Overdue purging alerts, Legal holds active, Storage usage trends
- **Acceptance Criteria**:
  - AC-1: Real-time metrics
  - AC-2: Export compliance reports
  - AC-3: Alert on retention policy violations

---

## 3. Yêu cầu kỹ thuật

| Component              | File/Package                              | Thay đổi                          |
|------------------------|-------------------------------------------|-----------------------------------|
| Retention Policy Store | `backend/store/retention_policy.go`       | Policy CRUD                       |
| Purge Runner           | `backend/runner/purger/`                  | Extends existing cleaner runner   |
| Legal Hold Service     | `backend/api/v1/legal_hold_service.go`    | Legal hold management             |
| Retention Dashboard    | `frontend/src/views/RetentionDashboard.tsx`| Compliance dashboard             |
| Feature Gate           | `enterprise/feature.go`                   | `FeatureDataRetention`            |

---

## 4. Rollout Plan

| Phase   | Mô tả                                  | Timeline       |
|---------|------------------------------------------|----------------|
| Phase 1 | Retention policy CRUD + configuration    | Sprint 1       |
| Phase 2 | Automated purging engine                 | Sprint 2       |
| Phase 3 | Legal hold mechanism                     | Sprint 3       |
| Phase 4 | Compliance dashboard                     | Sprint 3       |
