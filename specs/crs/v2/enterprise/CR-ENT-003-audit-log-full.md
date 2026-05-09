# Change Request: Audit Log (Full)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-003                                               |
| **Feature ID**     | SEC-10                                                   |
| **Title**          | Enterprise Audit Log — Full                              |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống **Audit Log đầy đủ** cho ENTERPRISE plan, ghi lại toàn bộ hoạt động trong workspace bao gồm database activities, user actions, configuration changes, và security events. Khác biệt với TEAM plan chỉ có Audit Log giới hạn (SEC-07).

### 1.2 Bối cảnh
| Plan       | Audit Log         |
|------------|--------------------|
| FREE       | —                  |
| TEAM       | Limited            |
| ENTERPRISE | Full               |

**TEAM (Limited)**: Chỉ log database change activities (Issue/Plan/Rollout).
**ENTERPRISE (Full)**: Log toàn bộ activities bao gồm API calls, auth events, admin actions, data access, policy changes.

### 1.3 Mục tiêu
- Ghi lại 100% database activities cho audit compliance
- Hỗ trợ SOC2, GDPR, HIPAA compliance requirements
- Cung cấp searchable, filterable audit trail
- Export audit logs cho external SIEM integration

---

## 2. Yêu cầu chức năng

### FR-001: Comprehensive Event Logging
- **Mô tả**: Ghi lại toàn bộ events theo categories.
- **Event Categories**:

| Category                 | Events                                                              | TEAM | ENT |
|--------------------------|---------------------------------------------------------------------|------|-----|
| **Database Changes**     | Issue create/update, Plan create, Rollout execute, Task run         | ✅   | ✅  |
| **Schema Operations**    | Schema migration, Schema sync, Schema dump                          | ✅   | ✅  |
| **SQL Execution**        | SQL query via Editor, Admin mode execution                          | ❌   | ✅  |
| **Data Access**          | Data export, Query result view (row count)                          | ❌   | ✅  |
| **Authentication**       | Login, Logout, SSO login, 2FA verify, API key usage                 | ❌   | ✅  |
| **Authorization**        | Permission denied, Role assignment, Grant/Revoke                     | ❌   | ✅  |
| **User Management**      | User create/deactivate, Group changes, SCIM sync                    | ❌   | ✅  |
| **Configuration**        | Setting changes, Policy updates, Environment changes                | ❌   | ✅  |
| **Instance Management**  | Instance create/update/delete, Connection test                       | ❌   | ✅  |
| **Project Management**   | Project create/archive, Member changes                              | ❌   | ✅  |
| **Security Events**      | Password change, 2FA enable/disable, Secret rotation                | ❌   | ✅  |

- **Acceptance Criteria**:
  - AC-1: Tất cả event categories được log cho ENTERPRISE
  - AC-2: Mỗi audit entry chứa: timestamp, actor, action, resource, result, IP address
  - AC-3: TEAM plan chỉ thấy Database Changes và Schema Operations
  - AC-4: Sensitive data (passwords, secrets) KHÔNG được log

### FR-002: Audit Log Entry Structure
- **Mô tả**: Mỗi audit log entry phải có cấu trúc chuẩn hóa.
- **Schema**:
  ```json
  {
    "id": "audit-xxxxx",
    "timestamp": "2026-05-08T10:00:00Z",
    "actor": {
      "type": "USER | SERVICE_ACCOUNT | WORKLOAD_IDENTITY | SYSTEM",
      "email": "user@example.com",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0..."
    },
    "action": "bytebase.v1.DatabaseService.CreateDatabase",
    "resource": {
      "type": "DATABASE | INSTANCE | PROJECT | USER | POLICY | SETTING",
      "name": "instances/prod-pg/databases/myapp",
      "project": "projects/my-project"
    },
    "request": {
      "method": "POST",
      "body_summary": "..."  
    },
    "response": {
      "status": "SUCCESS | DENIED | ERROR",
      "error_code": "PERMISSION_DENIED"
    },
    "severity": "INFO | WARNING | ERROR | CRITICAL",
    "metadata": {}
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Tất cả fields mandatory phải có giá trị
  - AC-2: IP address phải là client IP (xử lý X-Forwarded-For cho reverse proxy)
  - AC-3: Request body phải được sanitize (loại bỏ passwords, tokens)

### FR-003: Audit Log Query & Filter
- **Mô tả**: UI và API phải hỗ trợ query/filter audit logs.
- **Filter Dimensions**:
  - Time range (from/to)
  - Actor (user email, service account)
  - Action type (category)
  - Resource type / Resource name
  - Result (success/denied/error)
  - Severity level
  - IP address
  - Free-text search
- **Acceptance Criteria**:
  - AC-1: UI có advanced filter panel
  - AC-2: API `SearchAuditLogs` hỗ trợ tất cả filter dimensions
  - AC-3: Pagination với cursor-based approach cho large datasets
  - AC-4: Query performance < 2s cho 1M+ records

### FR-004: Audit Log Export
- **Mô tả**: Hỗ trợ export audit logs cho external SIEM/compliance tools.
- **Export Formats**:
  - JSON (structured)
  - CSV (spreadsheet-compatible)
- **Acceptance Criteria**:
  - AC-1: Export hỗ trợ filter (chỉ export filtered results)
  - AC-2: Export async cho large datasets (> 10K records)
  - AC-3: Export file có timestamp trong tên
  - AC-4: API endpoint cho programmatic export

### FR-005: Audit Log Retention
- **Mô tả**: Quản lý retention policy cho audit logs.
- **Logic**:
  - ENTERPRISE: Configurable retention (default: 365 days, max: unlimited)
  - TEAM: Fixed 90 days
- **Acceptance Criteria**:
  - AC-1: Admin có thể configure retention period
  - AC-2: `DataCleaner` runner tự động purge expired logs
  - AC-3: Warning khi logs sắp bị purge (30 days trước)

### FR-006: Audit Log Immutability
- **Mô tả**: Audit logs phải immutable — không thể sửa hoặc xóa thủ công.
- **Acceptance Criteria**:
  - AC-1: Không có API endpoint để update/delete audit logs
  - AC-2: Database level: REVOKE DELETE/UPDATE trên bảng audit_log cho application user
  - AC-3: Chỉ `DataCleaner` (system user) có thể purge expired logs

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                            | Thay đổi                                          |
|------------------------------|------------------------------------------|----------------------------------------------------|
| Audit Interceptor            | `backend/api/v1/audit.go`               | Mở rộng event categories cho ENTERPRISE            |
| Audit Log Service (gRPC)     | `backend/api/v1/audit_log_service.go`   | Thêm `SearchAuditLogs`, `ExportAuditLogs` RPCs    |
| Feature Gate                 | `enterprise/feature.go`                 | Define `FeatureFullAuditLog`                       |
| Store Layer                  | `backend/store/audit_log.go`            | Optimized queries, indexing, pagination            |
| Data Cleaner                 | `backend/runner/cleaner/`               | Retention-based purge logic                        |
| Auth Service                 | `backend/api/auth/`                     | Emit auth events cho audit                         |

### 3.2 Frontend Changes

| Component             | File                                        | Thay đổi                                  |
|-----------------------|---------------------------------------------|--------------------------------------------|
| Audit Log Page        | `frontend/src/views/AuditLog.vue`           | Full audit log viewer + filters           |
| Audit Log Filters     | `frontend/src/components/AuditFilter.vue`    | Advanced filter panel                      |
| Export Dialog         | `frontend/src/components/AuditExport.vue`    | Export configuration + download            |
| Settings              | `frontend/src/views/Settings.vue`           | Retention policy configuration             |

### 3.3 Database Changes

```sql
-- Indexes for performance (nếu chưa có)
CREATE INDEX idx_audit_log_created_ts ON audit_log (created_ts DESC);
CREATE INDEX idx_audit_log_actor ON audit_log (user_uid);
CREATE INDEX idx_audit_log_method ON audit_log (method);
CREATE INDEX idx_audit_log_resource ON audit_log (resource);
CREATE INDEX idx_audit_log_severity ON audit_log (severity);

-- Composite index cho common query patterns
CREATE INDEX idx_audit_log_search ON audit_log (created_ts DESC, user_uid, method);
```

### 3.4 Proto Changes

```protobuf
// backend/proto/v1/audit_log_service.proto
service AuditLogService {
  rpc SearchAuditLogs(SearchAuditLogsRequest) returns (SearchAuditLogsResponse);
  rpc ExportAuditLogs(ExportAuditLogsRequest) returns (ExportAuditLogsResponse);
}

message SearchAuditLogsRequest {
  string parent = 1;
  string filter = 2;       // CEL expression
  int32 page_size = 3;
  string page_token = 4;
  string order_by = 5;
}
```

---

## 4. Phụ thuộc

| Dependency            | Mô tả                                                       |
|-----------------------|---------------------------------------------------------------|
| License Service       | Xác định plan để gate audit log level                         |
| Auth Interceptor      | Cung cấp user context cho audit entries                       |
| Data Cleaner Runner   | Purge expired audit logs                                      |
| ACL Interceptor       | Audit log access cần permission `bb.auditLogs.search`        |

---

## 5. Security Considerations

| Concern                | Mitigation                                                    |
|------------------------|---------------------------------------------------------------|
| Sensitive data in logs | Sanitize request body — strip passwords, tokens, secrets     |
| Log tampering          | Immutable logs — no UPDATE/DELETE APIs                        |
| Log access control     | Only workspace admins can view audit logs                     |
| IP address spoofing    | Validate X-Forwarded-For chain, log direct connection IP      |
| Log volume DoS         | Rate limiting on export, async processing for large exports   |

---

## 6. Test Cases

| Test ID    | Mô tả                                                      | Expected Result                       |
|------------|--------------------------------------------------------------|---------------------------------------|
| TC-001     | ENTERPRISE: Login event được audit                          | Audit entry created với actor info    |
| TC-002     | ENTERPRISE: SQL query via Editor được audit                 | Audit entry with query metadata       |
| TC-003     | TEAM: SQL query via Editor KHÔNG được audit                 | No audit entry created                |
| TC-004     | ENTERPRISE: Permission denied event                         | Audit entry với severity=WARNING      |
| TC-005     | ENTERPRISE: Setting change event                            | Audit entry với before/after values   |
| TC-006     | Filter by actor + time range                                | Correct filtered results              |
| TC-007     | Export 100K records                                         | Async export, download link provided  |
| TC-008     | Attempt DELETE on audit_log via API                         | 405 Method Not Allowed                |
| TC-009     | Retention purge after configured period                     | Old logs purged, recent logs remain   |
| TC-010     | Password field NOT in audit log                             | Request body sanitized                |

---

## 7. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Extend Audit Interceptor             | Sprint 1       |
| Phase 2 | Search/Filter API + UI               | Sprint 2       |
| Phase 3 | Export functionality                  | Sprint 2       |
| Phase 4 | Retention policy management          | Sprint 3       |
| Phase 5 | Performance optimization + indexing  | Sprint 3       |
| Phase 6 | E2E testing + compliance validation  | Sprint 4       |
