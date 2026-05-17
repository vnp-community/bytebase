# TASK-SEC-016 — Emergency Access + Break-Glass Procedure

| Field            | Value                                      |
|------------------|---------------------------------------------|
| **Task ID**      | TASK-SEC-016                               |
| **Source**       | SOL-SEC-004 §3.4                           |
| **Status**       | Pending                                    |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 4                                   |

---

## Mô tả

Implement emergency access override (L4) với mandatory MFA re-authentication, time-limited (max 4h), full audit trail.

## Scope

1. **Migration**: `emergency_override` table (id, user_uid FK, justification, expires_at, created_ts, revoked)
2. **Service**: `RequestEmergencyAccess()` — verify 2FA, create time-limited override (max 240min), notify all admins
3. **ACL integration**: Check emergency override trước deny — nếu valid override exists → allow
4. **Audit**: Special audit category "EMERGENCY_ACCESS" cho tất cả actions during override

## Acceptance Criteria

- [ ] MFA required for emergency access
- [ ] Max duration 4h
- [ ] All workspace admins notified
- [ ] All actions during override audited
- [ ] Override auto-expires

## Files cần thay đổi

| File | Action |
|------|--------|
| `backend/migrator/migration/` | emergency_override table |
| `backend/api/v1/emergency_access.go` | New service |
| `backend/api/v1/acl.go` | Override check |

## Definition of Done

- Break-glass procedure documented and tested
