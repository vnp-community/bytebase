# TASK-ENT-004 — Audit Interceptor Event Category Expansion

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-004                               |
| **Source**       | SOL-ENT-003 (CR-ENT-003)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Mở rộng `AuditInterceptor` (L3) để log toàn bộ event categories cho ENTERPRISE plan: SQL execution, auth events, config changes, user management, security events.

## Scope

1. **Event Classification**: Implement `classifyMethod()` — phân loại gRPC methods theo category
2. **Plan-Based Filtering**: `shouldAudit()` — ENTERPRISE = all, TEAM = database change + schema ops, FREE = none
3. **New Categories**:
   - SQL Execution: `SQLService.Query`, `SQLService.AdminExecute`
   - Authentication: Login, Logout, SSO, 2FA events
   - Authorization: Permission denied events
   - User Management: CreateUser, DeactivateUser, GroupChanges
   - Configuration: Setting changes, Policy updates (before/after values)
   - Security Events: Password change, 2FA enable/disable
4. **Sanitization**: `sanitizeRequestBody()` — strip passwords, tokens, secrets, credentials, keys
5. **Feature Gate**: `FeatureFullAuditLog` trong `feature.go`

## Acceptance Criteria

- [x] Tất cả 6 event categories được log cho ENTERPRISE
- [x] TEAM chỉ log database changes và schema ops
- [x] Sensitive data NEVER logged (passwords, tokens, secrets)
- [x] IP logging: validate `X-Forwarded-For` chain
- [x] Unit tests cho `classifyMethod()`, `shouldAudit()`, `sanitizeRequestBody()`
- [x] Performance: interceptor overhead < 1ms per request

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/api/v1/audit.go` | Mở rộng event categories |
| `backend/enterprise/feature.go` | `FeatureFullAuditLog` |

## Definition of Done

- [x] All event categories captured
- [x] Sanitization verified — no secrets in logs
- [x] Performance benchmarked
