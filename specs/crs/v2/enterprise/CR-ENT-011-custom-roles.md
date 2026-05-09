# Change Request: Custom Roles

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-011                                               |
| **Feature ID**     | SEC-14                                                   |
| **Title**          | Custom Roles — User-Defined Permission Sets              |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Cho phép tạo **custom roles** với permission sets tùy chỉnh, bổ sung các predefined roles (workspaceAdmin, workspaceDBA, projectOwner, projectDeveloper, projectQuerier). Enterprise customers có thể define roles phù hợp với organizational structure.

### 1.2 Mục tiêu
- Tạo custom roles với fine-grained permissions
- Assign custom roles ở workspace và project level
- Permission inheritance và composition
- Audit trail cho role changes

---

## 2. Yêu cầu chức năng

### FR-001: Custom Role CRUD
- **Mô tả**: Admin có thể tạo, sửa, xóa custom roles.
- **Role Definition**:
  ```yaml
  role:
    name: "roles/dbReviewer"
    title: "Database Reviewer"
    description: "Can review and approve database changes"
    permissions:
      - bb.issues.get
      - bb.issues.list
      - bb.issues.update    # approve/reject
      - bb.plans.get
      - bb.plans.list
      - bb.planCheckRuns.list
      - bb.databases.getSchema
    scope: PROJECT          # WORKSPACE | PROJECT
  ```
- **Acceptance Criteria**:
  - AC-1: Custom role name must start with `roles/custom.`
  - AC-2: Permissions selected from available permission catalog
  - AC-3: Role scope determines where it can be assigned
  - AC-4: Cannot delete role if still assigned to users

### FR-002: Permission Catalog
- **Mô tả**: Hiển thị catalog of all available permissions.
- **Permission Categories**:
  - Database Management (`bb.databases.*`)
  - Instance Management (`bb.instances.*`)
  - Project Management (`bb.projects.*`)
  - Issue/Plan Management (`bb.issues.*`, `bb.plans.*`)
  - SQL Editor (`bb.sql.*`)
  - Schema Management (`bb.schemas.*`)
  - Policy Management (`bb.policies.*`)
  - IAM Management (`bb.iam.*`)
  - Audit Log (`bb.auditLogs.*`)
  - Settings (`bb.settings.*`)
- **Acceptance Criteria**:
  - AC-1: Permission catalog accessible trong role editor UI
  - AC-2: Permissions grouped by category
  - AC-3: Each permission has description
  - AC-4: Permission dependencies highlighted (e.g., `bb.issues.update` requires `bb.issues.get`)

### FR-003: Role Assignment
- **Mô tả**: Assign custom roles tới users và groups.
- **Acceptance Criteria**:
  - AC-1: Custom roles assignable via IAM Policy (same as predefined)
  - AC-2: Support assignment to: individual users, service accounts, groups
  - AC-3: Workspace-scope roles: assign in workspace IAM
  - AC-4: Project-scope roles: assign in project IAM
  - AC-5: CEL conditions supported (e.g., time-based access)

### FR-004: Role Comparison & Audit
- **Mô tả**: Tools để compare roles và audit role assignments.
- **Acceptance Criteria**:
  - AC-1: Compare 2 roles side-by-side (permission diff)
  - AC-2: View all users/groups with a specific role
  - AC-3: Role change history (audit log)

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                         | Thay đổi                                          |
|------------------------------|--------------------------------------|----------------------------------------------------|
| Role Service (gRPC)          | `backend/api/v1/role_service.go`     | CRUD custom roles                                  |
| IAM Manager                  | `backend/component/iam/`             | Resolve custom role permissions                    |
| Feature Gate                 | `enterprise/feature.go`              | Define `FeatureCustomRoles`                        |
| Store Layer                  | `backend/store/role.go`              | Custom role CRUD (already has `GetRoleSnapshot`)   |

### 3.2 Database Changes

```sql
-- role table already exists in Bytebase schema
-- Ensure custom roles are distinguished from predefined
ALTER TABLE role ADD COLUMN IF NOT EXISTS is_custom BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE role ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'PROJECT';
```

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Create custom role with selected permissions            | Role created successfully             |
| TC-002     | Assign custom role to user                              | User gets role's permissions          |
| TC-003     | User with custom role accesses permitted resource       | Access granted                        |
| TC-004     | User with custom role accesses non-permitted resource   | Access denied                         |
| TC-005     | Delete role that is still assigned                      | Error: role in use                    |
| TC-006     | Compare custom role vs predefined role                  | Permission diff displayed             |
| TC-007     | Non-ENTERPRISE: custom roles hidden                     | Feature gated                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Custom role CRUD backend             | Sprint 1       |
| Phase 2 | Permission catalog UI                | Sprint 1       |
| Phase 3 | IAM integration                      | Sprint 2       |
| Phase 4 | Role comparison + audit              | Sprint 3       |
