# Solution: CR-ENT-009 — Two-Factor Authentication (2FA)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-009                |
| **Solution**   | SOL-ENT-009               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Triển khai TOTP-based 2FA (RFC 6238) sử dụng `pquerna/otp` library (đã có trong codebase). Thêm 2FA challenge step vào login flow (L4 AuthService), encrypted TOTP secret storage, recovery codes, enforcement policy, và device trust mechanism.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L4 — Service** | `two_factor.go` (NEW) | TOTP setup, verify, recovery |
| **L4 — Service** | `auth/` | 2FA challenge in login flow |
| **L8 — Store** | `principal` table extensions | TOTP secrets, recovery codes |
| **L8 — Store** | `mfa_device_trust` (NEW) | Trusted device cookies |
| **L9 — Enterprise** | `feature.go` | `FeatureTwoFactorAuth` gate |
| **L1 — Presentation** | 2FA setup wizard, login challenge | QR code, TOTP input |

---

## 3. Chi tiết Implementation

### 3.1 TOTP Setup Flow

```go
func (s *TwoFactorService) SetupTOTP(ctx context.Context) (*v1pb.TOTPSetup, error) {
    key, _ := totp.Generate(totp.GenerateOpts{
        Issuer:      "Bytebase",
        AccountName: user.Email,
        Algorithm:   otp.AlgorithmSHA1,
        Digits:      otp.DigitsSix,
        Period:      30,
    })
    // Encrypt secret before storing
    encrypted := encrypt(key.Secret(), s.encryptionKey)
    // Generate 10 recovery codes
    recoveryCodes := generateRecoveryCodes(10)
    return &v1pb.TOTPSetup{
        QRCodeURI:     key.URL(),
        Secret:        key.Secret(),
        RecoveryCodes: recoveryCodes,
    }, nil
}
```

### 3.2 Login Flow Integration

```
Password OK → Check mfa_enabled
  → YES → Return 2FA challenge response (HTTP 200 + mfa_token)
    → User submits TOTP code + mfa_token
      → Verify TOTP (±1 period window)
      → OR verify recovery code (single-use, consume)
      → Check device trust cookie
        → Trusted → Skip 2FA
        → Not trusted → Require 2FA
  → NO → Normal session creation
```

### 3.3 Schema Migration

```sql
ALTER TABLE principal ADD COLUMN mfa_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE principal ADD COLUMN mfa_secret BYTEA;  -- AES-256-GCM encrypted
ALTER TABLE principal ADD COLUMN mfa_recovery_codes JSONB;  -- Encrypted

CREATE TABLE mfa_device_trust (
    id BIGSERIAL PRIMARY KEY,
    user_uid BIGINT NOT NULL REFERENCES principal(id),
    device_token TEXT NOT NULL UNIQUE,
    trusted_until TIMESTAMPTZ NOT NULL,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.4 Enforcement Policy

```go
// Settings: MFA_ENFORCEMENT = DISABLED | REQUIRED_FOR_ADMINS | REQUIRED_FOR_ALL
// Grace period: N days before enforcement kicks in
// SSO users exempt if IdP provides MFA
```

### 3.5 Security

- TOTP secret: AES-256-GCM encrypted at rest, never logged
- Recovery codes: Hashed with bcrypt, single-use
- Rate limiting: max 5 failed 2FA attempts per 5 min → temporary lockout
- Device trust cookie: HttpOnly, Secure, SameSite=Strict, HMAC-signed

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-010 | Password restrictions complement 2FA security |
| CR-ENT-003 | 2FA events logged to audit trail |
| CR-ENT-008 | SSO users may be exempt from 2FA |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | TOTP setup + verification | Sprint 1 |
| 2 | Login flow integration | Sprint 1 |
| 3 | Recovery codes | Sprint 2 |
| 4 | Enforcement policy | Sprint 2 |
| 5 | Device trust + admin management | Sprint 3 |
