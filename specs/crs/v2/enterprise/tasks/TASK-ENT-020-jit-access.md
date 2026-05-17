# TASK-ENT-020 — JIT (Just-In-Time) Access

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-020                               |
| **Source**       | SOL-ENT-017 (CR-ENT-017)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–2                                 |

---

## Mô tả

Enhance `AccessGrantService` hiện có để hỗ trợ full JIT access: request → approval → time-bound grant → auto-revocation.

## Scope

### Phase 1 — Sprint 1: Request + Approval + Time-Bound Binding
1. **Schema Migration**: `access_request` table — requester, resource, role, duration, justification, status, approver, expiry
2. **L4 — AccessGrantService Enhancement**: Access request CRUD + approval routing
3. **Self-Approval Prevention**: Requester cannot approve own request
4. **Approval Routing**: Configurable CEL rules → route to project owner / DBA
5. **L5 — IAM Time-Bound Binding**: `CheckPermission()` evaluates `ExpiresAt` on bindings

### Phase 2 — Sprint 2: Expiry Runner + Notifications
6. **L6 — JIT Expiry Runner (NEW)**: `runner/jitaccess/` — scan expired grants every 1 min, remove IAM bindings
7. **Notifications**: Notify user 15 min before grant expiry
8. **Extension Requests**: Allow requesting grant extension before expiry

## Acceptance Criteria

- [x] Access request CRUD functional
- [x] Approval routing via CEL rules works
- [x] Self-approval prevention enforced
- [x] Time-bound IAM binding created on approval (1h-7d)
- [x] IAM Manager respects `ExpiresAt` — expired bindings skipped
- [x] JIT expiry runner removes expired bindings every 1 min
- [x] User notified 15 min before grant expiry
- [x] Extension requests functional
- [x] All JIT events audited

## Dependencies

- CR-ENT-011 (Custom Roles) — JIT requests custom roles
- CR-ENT-012 (Data Masking) — JIT unmask grants for masked columns
- CR-ENT-003 (Audit Log) — all JIT events audited

## Definition of Done

- [x] JIT flow tested end-to-end (request → approve → use → expire → revoke)
- [x] Expiry runner accuracy verified
- [x] Notification delivery confirmed
