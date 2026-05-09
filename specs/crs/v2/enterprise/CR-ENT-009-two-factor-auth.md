# Change Request: Two-Factor Authentication (2FA)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-009                                               |
| **Feature ID**     | SEC-12                                                   |
| **Title**          | Two-Factor Authentication (TOTP)                         |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai xác thực hai yếu tố (2FA) sử dụng **TOTP (Time-Based One-Time Password)** theo RFC 6238. Admin có thể enforce 2FA cho toàn workspace hoặc specific roles.

### 1.2 Mục tiêu
- Hỗ trợ TOTP-based 2FA (Google Authenticator, Authy, etc.)
- Admin enforce 2FA cho specific user groups
- Recovery codes cho trường hợp mất device
- Audit logging cho 2FA events

---

## 2. Yêu cầu chức năng

### FR-001: TOTP Setup Flow
- **Mô tả**: User có thể enable 2FA trên account settings.
- **Flow**:
  ```
  User → Settings → Enable 2FA
    → Generate TOTP secret
    → Display QR code (otpauth:// URI)
    → User scan QR with authenticator app
    → User enter verification code
    → Generate recovery codes (10 single-use)
    → 2FA enabled
  ```
- **Acceptance Criteria**:
  - AC-1: QR code hiển thị otpauth URI theo standard format
  - AC-2: Secret key hiển thị dạng text (fallback cho manual entry)
  - AC-3: Verification code required trước khi finalize enable
  - AC-4: 10 recovery codes generated (single-use, 16-char alphanumeric)
  - AC-5: Recovery codes only shown once, user must confirm saved

### FR-002: 2FA Login Flow
- **Mô tả**: Login flow yêu cầu 2FA code sau password verification.
- **Flow**:
  ```
  Login → Password OK → 2FA Challenge
    → User enters TOTP code (6 digits) OR recovery code
    → Verify → Session created
  ```
- **Acceptance Criteria**:
  - AC-1: TOTP window tolerance: ±1 period (30s each side)
  - AC-2: Recovery code single-use (consumed after verification)
  - AC-3: Rate limiting: max 5 failed attempts per 5 minutes → temporary lockout
  - AC-4: "Remember this device" option (30 days via device cookie)

### FR-003: 2FA Enforcement Policy
- **Mô tả**: Admin có thể enforce 2FA cho workspace.
- **Enforcement Options**:
  - `DISABLED` — 2FA is optional
  - `REQUIRED_FOR_ADMINS` — Required cho workspace admins/DBAs
  - `REQUIRED_FOR_ALL` — Required cho tất cả users
- **Acceptance Criteria**:
  - AC-1: Users under enforcement phải setup 2FA before next action
  - AC-2: Grace period configurable (e.g., 7 days before enforcement)
  - AC-3: SSO users exempt from 2FA if IdP provides MFA

### FR-004: 2FA Management
- **Mô tả**: Admin actions cho 2FA management.
- **Actions**:
  - View 2FA status per user
  - Reset 2FA for user (admin action)
  - Regenerate recovery codes
  - Disable 2FA
- **Acceptance Criteria**:
  - AC-1: Admin can reset user's 2FA (requires admin password confirm)
  - AC-2: User can regenerate recovery codes (requires TOTP verify)
  - AC-3: Disable 2FA requires TOTP or recovery code verification

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                       | Thay đổi                                          |
|------------------------------|------------------------------------|----------------------------------------------------|
| 2FA Service                  | `backend/api/v1/two_factor.go`     | TOTP setup, verify, recovery                       |
| Auth Service                 | `backend/api/auth/`                | 2FA challenge in login flow                        |
| Feature Gate                 | `enterprise/feature.go`            | Define `FeatureTwoFactorAuth`                      |
| Store Layer                  | `backend/store/`                   | Store encrypted TOTP secrets + recovery codes      |

### 3.2 Database Changes

```sql
ALTER TABLE principal ADD COLUMN mfa_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE principal ADD COLUMN mfa_secret BYTEA;  -- Encrypted TOTP secret
ALTER TABLE principal ADD COLUMN mfa_recovery_codes JSONB;  -- Encrypted recovery codes

CREATE TABLE mfa_device_trust (
    id BIGSERIAL PRIMARY KEY,
    user_uid BIGINT NOT NULL REFERENCES principal(id),
    device_token TEXT NOT NULL UNIQUE,
    trusted_until TIMESTAMPTZ NOT NULL,
    created_ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 4. Security Considerations

| Concern                | Mitigation                                                    |
|------------------------|---------------------------------------------------------------|
| TOTP secret exposure   | Encrypt at rest (AES-256-GCM), never log                    |
| Recovery code brute    | Rate limiting + account lockout after N failures              |
| Replay attacks         | TOTP with ±1 window, mark used codes                         |
| Device trust cookie    | HttpOnly, Secure, SameSite=Strict, signed HMAC              |
| Admin reset abuse      | Require admin's own password + audit log                     |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | Enable 2FA → QR code generated                          | Valid otpauth URI                     |
| TC-002     | Login with valid TOTP code                              | Session created                       |
| TC-003     | Login with expired TOTP code                            | Rejected                             |
| TC-004     | Login with recovery code                                | Success, code consumed               |
| TC-005     | 6 failed 2FA attempts                                   | Temporary lockout                    |
| TC-006     | "Remember device" — skip 2FA for 30 days                | 2FA not prompted                     |
| TC-007     | Enforcement: user without 2FA tries to access           | Forced to setup 2FA                  |
| TC-008     | Admin reset user's 2FA                                   | User's 2FA disabled, must re-setup   |
| TC-009     | Non-ENTERPRISE plan: 2FA settings hidden                 | Feature gated                        |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | TOTP setup + verification            | Sprint 1       |
| Phase 2 | Login flow integration               | Sprint 1       |
| Phase 3 | Recovery codes                       | Sprint 2       |
| Phase 4 | Enforcement policy                   | Sprint 2       |
| Phase 5 | Device trust + admin management      | Sprint 3       |
