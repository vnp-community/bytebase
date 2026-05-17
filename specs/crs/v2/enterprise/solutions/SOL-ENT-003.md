# Solution: CR-ENT-003 — Audit Log (Full)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-003                |
| **Solution**   | SOL-ENT-003               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Mở rộng `AuditInterceptor` hiện có (L3) để log **toàn bộ event categories** cho ENTERPRISE plan, bao gồm SQL execution, auth events, config changes. Thêm `SearchAuditLogs`/`ExportAuditLogs` RPCs, optimized indexing, configurable retention, và immutability enforcement.

---

## 2. Architectural Alignment

```
L2 API Gateway ──► L3 Security Layer (Audit Interceptor) ──► L8 Store (audit_log)
                         │                                         ▲
                         ├── Feature Gate (L9) ◄──────────────────┘
                         │
L4 Service ◄──── SearchAuditLogs / ExportAuditLogs APIs
                         │
L6 Runner ◄──── DataCleaner (retention purge)
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L3 — Security** | `backend/api/v1/audit.go` (25KB) | Mở rộng event categories dựa trên plan |
| **L4 — Service** | `audit_log_service.go` | `SearchAuditLogs`, `ExportAuditLogs` RPCs |
| **L8 — Store** | `store/audit_log.go` | Optimized queries + pagination + indexes |
| **L6 — Runner** | `runner/cleaner/` | Retention-based purge (365 days ENT, 90 days TEAM) |
| **L9 — Enterprise** | `feature.go` | `FeatureFullAuditLog` gate |
| **L1 — Presentation** | `AuditLog.vue`, `AuditFilter.vue`, `AuditExport.vue` | Full UI |

---

## 3. Chi tiết Implementation

### 3.1 L3 — Audit Interceptor Enhancement

**File**: `backend/api/v1/audit.go`

Hiện tại AuditInterceptor đã log database change activities. Mở rộng bằng cách thêm event category resolution:

```go
func (a *AuditInterceptor) shouldAudit(ctx context.Context, method string) bool {
    category := classifyMethod(method) // DATABASE_CHANGE, SQL_EXECUTION, AUTH, CONFIG, etc.

    plan := a.licenseService.GetCurrentPlan(ctx)
    switch plan {
    case ENTERPRISE:
        return true  // Log everything
    case TEAM:
        return category == DATABASE_CHANGE || category == SCHEMA_OPERATION
    default:
        return false
    }
}
```

**Event Categories mới** (ENTERPRISE-only):

| Category | Methods | Implementation |
|----------|---------|---------------|
| SQL Execution | `SQLService.Query`, `SQLService.AdminExecute` | Log query metadata (DB, row count), **NOT** query results |
| Authentication | Login, Logout, SSO, 2FA events | Emit từ `AuthService` |
| Authorization | Permission denied events | Emit từ `ACL Interceptor` |
| User Management | CreateUser, DeactivateUser, GroupChanges | Standard interceptor |
| Configuration | Setting changes, Policy updates | Capture before/after values |
| Security Events | Password change, 2FA enable/disable | Emit từ respective services |

### 3.2 Audit Entry Sanitization

```go
func sanitizeRequestBody(body proto.Message) string {
    // Deep-copy and strip sensitive fields
    sensitiveFields := []string{"password", "secret", "token", "credential", "key"}
    sanitized := proto.Clone(body)
    stripFields(sanitized, sensitiveFields)
    return protojson.Format(sanitized)
}
```

### 3.3 L4 — Search & Export APIs

**Proto**: `audit_log_service.proto`

```protobuf
service AuditLogService {
  rpc SearchAuditLogs(SearchAuditLogsRequest) returns (SearchAuditLogsResponse);
  rpc ExportAuditLogs(ExportAuditLogsRequest) returns (ExportAuditLogsResponse);
}

message SearchAuditLogsRequest {
  string parent = 1;              // workspaces/{workspace}
  string filter = 2;              // CEL expression for filtering
  int32 page_size = 3;            // max 1000
  string page_token = 4;          // cursor-based pagination
  string order_by = 5;            // default: created_ts DESC
}
```

**CEL filter examples**:
```
actor.email == "admin@example.com" && created_ts > timestamp("2026-01-01T00:00:00Z")
method.startsWith("bytebase.v1.SQLService") && response.status == "DENIED"
```

### 3.4 L8 — Database Indexes

```sql
-- Performance indexes (migration file)
CREATE INDEX idx_audit_log_created_ts ON audit_log (created_ts DESC);
CREATE INDEX idx_audit_log_actor ON audit_log (user_uid);
CREATE INDEX idx_audit_log_method ON audit_log (method);
CREATE INDEX idx_audit_log_resource ON audit_log (resource);

-- Composite index for common query patterns
CREATE INDEX idx_audit_log_search ON audit_log (created_ts DESC, user_uid, method);
```

### 3.5 L6 — Retention Purge

**File**: `backend/runner/cleaner/audit_cleaner.go`

```go
func (c *DataCleaner) purgeExpiredAuditLogs(ctx context.Context) error {
    retentionDays := c.getAuditRetention(ctx) // ENT: 365, TEAM: 90
    cutoff := time.Now().AddDate(0, 0, -retentionDays)

    _, err := c.store.PurgeAuditLogsBefore(ctx, cutoff)
    return err
}
```

### 3.6 Immutability Enforcement

- **API level**: Không expose `UpdateAuditLog` / `DeleteAuditLog` endpoints.
- **Database level**: Application user chỉ có `INSERT` và `SELECT` trên `audit_log`.
- **Purge**: Chỉ `DataCleaner` runner (system context) có quyền `DELETE`.

---

## 4. Database Changes

```sql
-- Migration: Add indexes for audit log search performance
CREATE INDEX IF NOT EXISTS idx_audit_log_created_ts ON audit_log (created_ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (user_uid);
CREATE INDEX IF NOT EXISTS idx_audit_log_method ON audit_log (method);
CREATE INDEX IF NOT EXISTS idx_audit_log_search ON audit_log (created_ts DESC, user_uid, method);
```

---

## 5. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Sensitive data in logs | `sanitizeRequestBody()` strips passwords, tokens, secrets |
| Log tampering | Immutable: no UPDATE/DELETE APIs |
| Log access control | Permission `bb.auditLogs.search` required |
| IP spoofing | Validate `X-Forwarded-For` chain, log direct connection IP |
| Log volume DoS | Rate limiting on export, async processing >10K records |

---

## 6. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-006 (Risk Assessment) | Audit log captures risk assessment events |
| CR-ENT-009 (2FA) | 2FA events logged to audit trail |
| CR-ENT-008 (SSO) | SSO login events logged |

---

## 7. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | Extend Audit Interceptor event categories | Sprint 1 |
| 2 | Search/Filter API + UI | Sprint 2 |
| 3 | Export functionality (JSON/CSV) | Sprint 2 |
| 4 | Retention policy management | Sprint 3 |
| 5 | Performance optimization + indexing | Sprint 3 |
| 6 | E2E testing + compliance validation | Sprint 4 |
