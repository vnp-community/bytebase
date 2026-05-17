# TASK-ENT-013 — Password Policy Engine & Account Lockout

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-013                               |
| **Source**       | SOL-ENT-010 (CR-ENT-010)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Triển khai configurable password policy engine tại L4 (AuthService): complexity rules, rotation policy, password history, account lockout. Tuân thủ NIST SP 800-63B.

## Scope

### Phase 1 — Sprint 1: Complexity Rules + UI
1. **L4 — PasswordValidator (NEW)**: `password_policy.go` — validate minLength, requireUppercase, requireLowercase, requireDigits, requireSpecial, disallowUsername, disallowCommon
2. **Common Password Check**: Built-in list of common passwords
3. **L1 — Frontend**: Real-time password strength indicator
4. **L9 — Feature Gate**: `FeaturePasswordRestrictions`

### Phase 2 — Sprint 2: Rotation + History
5. **Schema Migration**: Add `password_changed_at`, `failed_login_attempts`, `locked_until` to `principal`; create `password_history` table
6. **Rotation Policy**: Warning banner N days before expiry, force redirect when expired
7. **Password History**: Prevent reuse of last N passwords
8. **Service Account Exemption**: Service accounts exempt from rotation

### Phase 3 — Sprint 3: Account Lockout
9. **Lockout Logic**: Increment `failed_login_attempts`, lock after `MaxAttempts`, lockout for configurable duration
10. **Audit Events**: `ACCOUNT_LOCKED` event emitted
11. **L6 — Runner**: Cleanup expired password history entries

## Acceptance Criteria

- [x] All complexity rules configurable and enforced
- [x] Common password detection works
- [x] Frontend strength indicator real-time
- [x] Rotation warning displayed before expiry
- [x] Force password change when expired
- [x] Password history prevents reuse of last N passwords
- [x] Account lockout after max failed attempts
- [x] Lockout duration configurable
- [x] Auto-unlock after lockout period
- [x] Service accounts exempt from rotation
- [x] NIST SP 800-63B compliance

## Dependencies

- CR-ENT-009 (2FA) — defense-in-depth
- CR-ENT-003 (Audit Log) — lockout events logged

## Definition of Done

- [x] All policy rules tested with edge cases
- [x] Lockout/unlock flow verified
- [x] NIST compliance checklist completed
