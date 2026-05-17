# Change Request: User Activity Privacy

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PRV-008                                               |
| **Feature ID**     | UR-S03, ADM-07 (extends), NF-SE04                       |
| **Title**          | User Activity Privacy                                    |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Related CRs**    | CR-ENT-021 (Watermark), CR-PRV-005                      |

---

## 1. Tổng quan

### 1.1 Mô tả
Bảo vệ **quyền riêng tư của người sử dụng Bytebase** — đảm bảo hoạt động cá nhân (query history, login patterns, browsing behavior) không bị expose không cần thiết cho admin/DBA. Cân bằng giữa nhu cầu audit/compliance và quyền riêng tư người dùng.

### 1.2 Bối cảnh từ PRD/URD
- **PRD SQL-10**: Query History — admin có thể xem toàn bộ query history của mọi user
- **PRD ADM-07**: Watermark — chứa user identity → privacy concern
- **URD NF-SE04**: Session management — session data chứa activity patterns
- **URD UR-S03**: Full audit log — mọi API call → user behavior profiling risk

### 1.3 Mục tiêu
- User-scoped query history (chỉ user thấy history của mình, trừ audit purpose)
- Activity data minimization (chỉ collect cần thiết cho chức năng)
- Privacy-respecting watermark (không chứa PII trực tiếp)
- User privacy settings (opt-out cho non-essential tracking)
- Admin access to user data requires justification + audit trail

---

## 2. Yêu cầu chức năng

### FR-001: User-Scoped Activity Data
- **Mô tả**: Giới hạn visibility của user activity data.
- **Scoping Rules**:

| Data Type         | User Visibility | Admin Visibility | DBA Visibility   |
|-------------------|----------------|------------------|------------------|
| Query History     | Own only       | With justification| Own project only |
| Login History     | Own only       | Aggregated stats | ❌               |
| Session Data      | Own only       | ❌               | ❌               |
| SQL Worksheets    | Own + shared   | With justification| Own project only |
| Export History    | Own only       | Aggregated stats | Own project only |

- **Acceptance Criteria**:
  - AC-1: Default: user data only visible to owner
  - AC-2: Admin access requires explicit justification (logged)
  - AC-3: Aggregated/anonymized views for management dashboards
  - AC-4: User notification khi admin accesses their data

### FR-002: Activity Data Minimization
- **Mô tả**: Thu thập tối thiểu dữ liệu hoạt động cần thiết (GDPR Article 5(1)(c)).
- **Minimization Rules**:
  - Query text: store hash + structure, not full literal values
  - Login: store timestamp + success/fail, not IP geolocation by default
  - Session: store duration, not click-by-click activity
  - Telemetry: opt-in only, anonymized before collection
- **Acceptance Criteria**:
  - AC-1: Telemetry collection requires user consent
  - AC-2: Data minimization configurable per workspace
  - AC-3: Privacy impact assessment for new data collection

### FR-003: Privacy-Respecting Watermark
- **Mô tả**: Watermark (ADM-07) không chứa PII trực tiếp.
- **Current**: Watermark hiển thị email/username → PII exposure risk
- **Proposed**: Watermark sử dụng pseudonymized user token + timestamp
  - Token traceable back to user only bởi admin (with audit)
  - Steganographic watermark option (invisible to users)
- **Acceptance Criteria**:
  - AC-1: Default watermark không hiển thị email/full name
  - AC-2: Configurable watermark content (token, initials, department)
  - AC-3: Watermark trace-back requires admin approval

### FR-004: User Privacy Settings
- **Mô tả**: User có thể quản lý privacy preferences.
- **Settings**:
  - Query history retention (1 day, 7 days, 30 days, 90 days)
  - Telemetry opt-out
  - Activity visibility (private / team / project)
  - Login notification preferences
  - Data download (export own data — GDPR portability)
- **Acceptance Criteria**:
  - AC-1: Privacy settings accessible from user profile
  - AC-2: Settings take effect within 24 hours
  - AC-3: Default settings: most privacy-preserving option

---

## 3. Yêu cầu kỹ thuật

| Component               | File/Package                                | Thay đổi                          |
|--------------------------|---------------------------------------------|-----------------------------------|
| User Privacy Service     | `backend/api/v1/user_privacy_service.go`   | Privacy settings CRUD             |
| Activity Scoping         | `backend/store/activity.go`                 | Scoped activity queries           |
| Watermark Privacy        | `backend/component/watermark/`              | Pseudonymized watermarks          |
| Data Minimization        | `backend/api/v1/sql_service.go`             | Query text minimization           |
| Privacy Settings UI      | `frontend/src/views/UserPrivacy.tsx`        | User privacy preferences          |
| Feature Gate             | `enterprise/feature.go`                     | `FeatureUserPrivacy`              |

---

## 4. Rollout Plan

| Phase   | Mô tả                                   | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | User-scoped activity data                 | Sprint 1       |
| Phase 2 | Activity data minimization                | Sprint 2       |
| Phase 3 | Privacy-respecting watermark              | Sprint 2       |
| Phase 4 | User privacy settings UI                  | Sprint 3       |
