# Change Request: Periodic Access Review Automation

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-004                                               |
| **Gap ID**         | G4                                                       |
| **Title**          | Periodic Access Review Automation                        |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Depends On**     | CR-INS-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Module tự động hóa quy trình rà soát quyền truy cập database định kỳ (Periodic Access Review). Quét privilege assignments, so sánh với permission matrix chuẩn (VNPAY 6 nhóm quyền), phát hiện violations, và tạo remediation issues.

### 1.2 Bối cảnh
Gap G4 yêu cầu periodic access review. Hiện đề xuất Steampipe — nhưng Bytebase đã có kết nối tới tất cả DB instances. Tích hợp access review vào Bytebase giảm tool sprawl và leverage existing RBAC data.

### 1.3 Mục tiêu
- Scheduled access review (weekly/monthly/quarterly)
- Permission matrix compliance check cross-engine
- Privilege escalation detection
- Auto-create REVOKE issues cho violations
- Compliance report generation (PCI-DSS / ISO27001)

---

## 2. Yêu cầu chức năng

### FR-001: Permission Matrix Definition
- Define expected permission matrix trong Bytebase:
  - User groups: Admin, Owner, Service, Operate, Monitor, Viewer
  - Allowed privileges per group per engine
  - Disallowed privilege combinations
- Import matrix từ CSV/YAML
- Version-controlled matrix changes

### FR-002: Privilege Scanner
Scan actual privileges trên mỗi engine:

| Engine | Queries |
|---|---|
| Oracle | `dba_sys_privs`, `dba_tab_privs`, `dba_role_privs` |
| PostgreSQL | `pg_roles`, `information_schema.role_table_grants`, `pg_auth_members` |
| MySQL | `mysql.user`, `mysql.db`, `mysql.tables_priv` |
| SQL Server | `sys.database_permissions`, `sys.server_permissions` |
| MongoDB | `db.getUsers()`, `db.getRoles()` |

### FR-003: Violation Detection Engine
- Compare actual privileges vs expected matrix
- Detect violations:
  - **Excessive privileges**: User có quyền vượt matrix
  - **Orphaned users**: DB users không map với Bytebase/LDAP users
  - **Superuser accounts**: Unexpected DBA/superuser privileges
  - **Stale permissions**: Users inactive > 90 days vẫn có privileges
  - **Cross-schema access**: Unauthorized cross-schema grants

### FR-004: Review Workflow
- Scheduled review → auto-generate Review Report
- Review Report assigned to DBA Lead
- DBA có thể: Approve (acknowledge exception), Revoke (create Issue), Defer
- Revoke → auto-create Issue với REVOKE SQL
- Exception tracking: tại sao user cần excess privilege

### FR-005: Access Review Dashboard
- Matrix compliance heatmap (instances × user groups)
- Violation trend over time
- Top violating users/instances
- Review completion tracking
- Export: PDF report cho compliance audit

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Permission Matrix Store | `backend/store/permission_matrix.go` | Matrix CRUD |
| Privilege Scanner | `backend/component/accessreview/scanner.go` | Cross-engine scanning |
| Violation Detector | `backend/component/accessreview/detector.go` | Matrix comparison |
| Review Workflow | `backend/component/accessreview/workflow.go` | Review lifecycle |
| Scanner Plugins | `backend/plugin/db/*/privilege_query.go` | Engine-specific queries |
| Review API | `backend/api/v1/access_review_service.go` | API endpoints |
| Matrix Editor UI | `frontend/src/views/AccessReview/MatrixEditor.vue` | Matrix config |
| Review Dashboard | `frontend/src/views/AccessReview/Dashboard.vue` | Heatmap + reports |

---

## 4. Test Cases

| Test ID | Mô tả | Expected |
|---|---|---|
| TC-001 | User có DBA privilege nhưng matrix = Viewer | Violation detected |
| TC-002 | Scheduled review → report generated | Review assigned to DBA |
| TC-003 | DBA clicks Revoke → Issue created | REVOKE SQL in pipeline |
| TC-004 | Exception approved → no future alerts | Tracked in exceptions |
| TC-005 | Orphaned PG user detected | Flagged in violations |
| TC-006 | Compliance report exported as PDF | Valid PCI-DSS format |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Permission matrix + privilege scanner | Sprint 1-2 |
| Phase 2 | Violation detection + dashboard | Sprint 3 |
| Phase 3 | Review workflow + auto-remediation | Sprint 4-5 |
| Phase 4 | Compliance reporting | Sprint 6 |
