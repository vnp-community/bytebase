# Change Request: Privilege Escalation Prevention

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-005                                               |
| **Feature ID**     | SEC-01, SEC-14                                           |
| **Title**          | Privilege Escalation Prevention                          |
| **Plan**           | ALL                                                      |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống phòng chống leo thang đặc quyền (privilege escalation) toàn diện: role assignment validation, permission boundary enforcement, self-elevation prevention, và separation of duties.

### 1.2 Bối cảnh
Bytebase IAM (SEC-01) với Custom Roles (SEC-14) cần hardening để ngăn chặn: user tự nâng quyền, admin gán quyền vượt scope, horizontal privilege escalation giữa projects.

---

## 2. Yêu cầu chức năng

### FR-001: Role Assignment Validation
- User chỉ có thể gán role ≤ own role level
- **Acceptance Criteria**:
  - AC-1: Role hierarchy enforcement — không thể gán role cao hơn mình
  - AC-2: Custom role creation — chỉ admin có thể tạo roles với quyền ≤ admin's permissions
  - AC-3: Dual-admin approval cho workspace-level role changes
  - AC-4: Audit log cho mọi role assignment/revocation

### FR-002: Permission Boundary
- Maximum permission set mà user có thể receive, bất kể role assignments
- **Acceptance Criteria**:
  - AC-1: Permission boundary set per user hoặc per group
  - AC-2: Effective permission = intersection(role_permissions, boundary)
  - AC-3: Boundary violations logged as security events
  - AC-4: Admin UI hiển thị effective permissions sau boundary

### FR-003: Self-Elevation Prevention
- **Acceptance Criteria**:
  - AC-1: User không thể modify own roles/permissions
  - AC-2: User không thể modify own permission boundary
  - AC-3: Super admin changes require second admin approval
  - AC-4: Service accounts không thể elevate own scope

### FR-004: Cross-Project Isolation
- **Acceptance Criteria**:
  - AC-1: Project-level roles không leak sang projects khác
  - AC-2: Database queries scoped tới assigned projects only
  - AC-3: Audit log isolation — users chỉ thấy logs của assigned projects
  - AC-4: Schema search results filtered by project membership

### FR-005: Separation of Duties
- **Acceptance Criteria**:
  - AC-1: Same user không thể create AND approve same change
  - AC-2: DBA role separated from security admin role
  - AC-3: Configurable SoD policies per workflow
  - AC-4: SoD violation detection and alerting

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| IAM Component                | `backend/component/iam/`                    | Permission boundary, hierarchy enforcement  |
| Role Service                 | `backend/api/v1/role_service.go`            | Assignment validation, SoD checks           |
| ACL Interceptor              | `backend/api/interceptor/acl.go`            | Cross-project isolation enforcement         |
| Approval Runner              | `backend/runner/approval/`                  | SoD validation in approval flow             |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Editor tries to assign Admin role                    | Permission denied                |
| TC-002  | User modifies own role                               | Self-elevation blocked           |
| TC-003  | Project member queries cross-project data            | Access denied                    |
| TC-004  | Same user creates and approves change                | SoD violation blocked            |
| TC-005  | Permission boundary restricts role                   | Effective permission reduced     |
| TC-006  | Custom role with permissions exceeding creator        | Creation blocked                 |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Role hierarchy enforcement           | Sprint 1       |
| Phase 2 | Permission boundary                  | Sprint 2       |
| Phase 3 | Self-elevation + cross-project       | Sprint 2       |
| Phase 4 | Separation of duties                 | Sprint 3       |
