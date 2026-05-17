# TASK-ENT-012 — Two-Factor Authentication (TOTP)

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-012                               |
| **Source**       | SOL-ENT-009 (CR-ENT-009)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Triển khai TOTP-based 2FA (RFC 6238) với encrypted secret storage, recovery codes, enforcement policy, và device trust mechanism.

## Scope

### Phase 1 — Sprint 1: TOTP Setup + Verification
1. **L4 — TwoFactorService (NEW)**: `SetupTOTP()` — generate TOTP key (SHA1, 6 digits, 30s period), encrypt secret (AES-256-GCM), generate 10 recovery codes
2. **Login Flow Integration**: Password OK → check `mfa_enabled` → 2FA challenge → verify TOTP (±1 period window) OR verify recovery code (single-use)
3. **L1 — Frontend**: 2FA setup wizard (QR code display), login challenge (TOTP input)

### Phase 2 — Sprint 2: Recovery + Enforcement
4. **Recovery Codes**: Hashed with bcrypt, single-use, consume on use
5. **Enforcement Policy**: `MFA_ENFORCEMENT` setting: DISABLED | REQUIRED_FOR_ADMINS | REQUIRED_FOR_ALL
6. **Grace Period**: N days before enforcement kicks in
7. **SSO Exemption**: SSO users exempt if IdP provides MFA

### Phase 3 — Sprint 3: Device Trust + Admin Management
8. **Schema Migration**: Add `mfa_enabled`, `mfa_secret`, `mfa_recovery_codes` to `principal`; create `mfa_device_trust` table
9. **Device Trust**: HttpOnly, Secure, SameSite=Strict cookie, HMAC-signed, trusted for configurable period
10. **Admin Management**: Admin can reset user's 2FA, view 2FA enrollment status
11. **Rate Limiting**: Max 5 failed 2FA attempts per 5 min → temporary lockout

## Acceptance Criteria

- [x] TOTP setup generates valid QR code
- [x] TOTP verification works with ±1 period window
- [x] TOTP secret encrypted at rest (AES-256-GCM), never logged
- [x] Recovery codes: 10 codes generated, bcrypt hashed, single-use
- [x] Enforcement policy works for all 3 modes
- [x] Grace period countdown functional
- [x] SSO users correctly exempted
- [x] Device trust cookie functional (skip 2FA on trusted devices)
- [x] Rate limiting: lockout after 5 failed attempts
- [x] Admin can reset user's 2FA

## Dependencies

- CR-ENT-010 (Password Restrictions) — defense-in-depth
- CR-ENT-003 (Audit Log) — 2FA events logged
- CR-ENT-008 (SSO) — SSO exemption logic

## Definition of Done

- [x] TOTP flow tested with Google Authenticator / Authy
- [x] Security requirements verified (encryption, rate limiting)
- [x] Enforcement policy E2E tested
