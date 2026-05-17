# Change Request: DB Password Policy Lifecycle Manager

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-INS-001                                               |
| **Gap ID**         | G1                                                       |
| **Title**          | DB Password Policy Lifecycle Manager                     |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-15                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Xây dựng module quản lý **password policy lifecycle cho database user** trực tiếp trong Bytebase. Hiện tại Bytebase chỉ quản lý password policy cho chính Bytebase platform (SEC-13), không quản lý password policy trên target databases. Module này sẽ mở rộng khả năng Bytebase để:
- Định nghĩa, áp dụng, và giám sát password profile trên mỗi DB engine
- Tự động sinh SQL tạo/áp dụng profile phù hợp từng engine
- Theo dõi compliance status của tất cả DB users

### 1.2 Bối cảnh
VNPAY yêu cầu chuẩn hóa password policy (expiry 90 ngày, complexity, history, lockout) trên 90+ dịch vụ, 5 DB engines. Hiện tại phải dùng native DB features (`CREATE PROFILE` Oracle, `pg_password_policy` PG, `validate_password` MySQL) thủ công. Cần centralized management.

### 1.3 Mục tiêu
- Centralized password policy definition cho all DB engines
- Auto-generate engine-specific SQL
- Policy compliance monitoring & enforcement
- Policy versioning & change tracking

---

## 2. Yêu cầu chức năng

### FR-001: Password Policy Template Registry
- Định nghĩa policy template với các thuộc tính:
  - `min_length`: 12+
  - `complexity`: uppercase + lowercase + digit + special
  - `max_age_days`: 90 (user) / UNLIMITED (service)
  - `history_count`: 5
  - `max_failed_attempts`: 5
  - `lockout_duration_minutes`: 60
  - `reuse_interval_days`: 365
- Hỗ trợ policy types: `USER_PROFILE`, `SERVICE_PROFILE`, `ADMIN_PROFILE`
- Template versioning với changelog

### FR-002: Engine-Specific SQL Generator
Tự động sinh SQL phù hợp cho mỗi engine từ policy template:

| Engine | Generated SQL |
|---|---|
| Oracle | `CREATE PROFILE ... LIMIT PASSWORD_LIFE_TIME ...` |
| PostgreSQL | `ALTER ROLE ... VALID UNTIL ...` + extension `pg_password_policy` setup |
| MySQL | `INSTALL COMPONENT 'file://component_validate_password'` + `SET GLOBAL validate_password.*` |
| SQL Server | `ALTER LOGIN ... CHECK_POLICY = ON, CHECK_EXPIRATION = ON` |
| MongoDB | Custom auth mechanism configuration via `db.updateUser()` |

### FR-003: Policy Assignment Manager
- Assign policy cho individual user hoặc group users
- Bulk assign qua Database Group
- Policy inheritance: Instance → Database → User level
- Override rules khi cần exemption

### FR-004: Policy Compliance Scanner
- Scheduled scan (configurable interval, default: daily)
- Scan tất cả registered instances
- Report users không tuân thủ policy:
  - Weak passwords (where detectable)
  - Missing profile assignment
  - Expired passwords not rotated
  - Users without proper profile
- Compliance score per instance, per project, overall

### FR-005: Policy Change Pipeline Integration
- Khi policy thay đổi, tự động tạo Issue/Plan để apply changes
- SQL generated → submitted cho approval workflow
- Rollout qua existing environment pipeline (Test → Staging → Prod)
- Changelog cập nhật tự động

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| Policy Template Store              | `backend/store/password_policy.go`                   | CRUD password policy templates               |
| Policy SQL Generator               | `backend/plugin/db/*/password_policy.go`             | Engine-specific SQL generation               |
| Compliance Scanner Service         | `backend/component/dbpolicy/scanner.go`              | Scheduled compliance scanning                |
| Policy Assignment Manager          | `backend/component/dbpolicy/assignment.go`           | User-policy mapping & inheritance            |
| Policy API (gRPC)                  | `backend/api/v1/password_policy_service.go`          | API endpoints for policy CRUD                |
| Policy UI — Template Editor        | `frontend/src/views/PasswordPolicy/TemplateEditor.vue` | Policy creation/edit UI                    |
| Policy UI — Compliance Dashboard   | `frontend/src/views/PasswordPolicy/Compliance.vue`   | Compliance status visualization              |
| Database Schema Migration          | `backend/migrator/migration/*/`                      | Tables: `password_policy`, `policy_assignment`, `compliance_scan` |

### 3.1 Database Schema

```sql
CREATE TABLE password_policy (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    policy_type VARCHAR(50) NOT NULL, -- USER_PROFILE, SERVICE_PROFILE, ADMIN_PROFILE
    min_length INT DEFAULT 12,
    require_uppercase BOOLEAN DEFAULT TRUE,
    require_lowercase BOOLEAN DEFAULT TRUE,
    require_digit BOOLEAN DEFAULT TRUE,
    require_special BOOLEAN DEFAULT TRUE,
    max_age_days INT DEFAULT 90,
    history_count INT DEFAULT 5,
    max_failed_attempts INT DEFAULT 5,
    lockout_duration_minutes INT DEFAULT 60,
    reuse_interval_days INT DEFAULT 365,
    version INT DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    creator_id INT REFERENCES principal(id)
);

CREATE TABLE policy_assignment (
    id SERIAL PRIMARY KEY,
    policy_id INT REFERENCES password_policy(id),
    scope_type VARCHAR(50) NOT NULL, -- INSTANCE, DATABASE, USER
    scope_id VARCHAR(255) NOT NULL,
    db_username VARCHAR(255),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by INT REFERENCES principal(id)
);

CREATE TABLE compliance_scan_result (
    id SERIAL PRIMARY KEY,
    scan_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    instance_id INT NOT NULL,
    db_username VARCHAR(255) NOT NULL,
    policy_id INT REFERENCES password_policy(id),
    is_compliant BOOLEAN NOT NULL,
    violations JSONB, -- array of violation codes
    details JSONB
);
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Policy SQL contains passwords    | Temporary passwords generated via secure random, stored in Vault |
| Compliance scan credentials      | Reuse existing Bytebase instance credentials (read-only)      |
| Policy modification audit        | Full audit log for all policy CRUD operations                 |
| Cross-engine consistency         | Validation layer ensures policy translates correctly per engine |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Tạo USER_PROFILE policy, generate Oracle SQL          | Valid `CREATE PROFILE` statement           |
| TC-002  | Tạo USER_PROFILE policy, generate PG SQL              | Valid `ALTER ROLE` + extension setup       |
| TC-003  | Assign policy cho instance, scan compliance           | Report non-compliant users                 |
| TC-004  | Thay đổi policy max_age → auto-create Issue           | New Issue in pipeline with updated SQL     |
| TC-005  | Scan instance with all users compliant                | 100% compliance score                      |
| TC-006  | Policy version bump → changelog updated               | Version history visible in UI              |
| TC-007  | Bulk assign policy via Database Group                 | All DBs in group receive assignment        |

---

## 6. Rollout Plan

| Phase   | Mô tả                                     | Timeline       |
|---------|--------------------------------------------|----------------|
| Phase 1 | Policy template CRUD + SQL generation      | Sprint 1-2     |
| Phase 2 | Assignment manager + pipeline integration  | Sprint 3       |
| Phase 3 | Compliance scanner + dashboard             | Sprint 4       |
| Phase 4 | Multi-engine validation + edge cases       | Sprint 5       |
