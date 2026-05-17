# Change Request: Brute-Force & Account Lockout Protection

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-003                                               |
| **Feature ID**     | SEC-12, SEC-13                                           |
| **Title**          | Brute-Force & Account Lockout Protection                 |
| **Plan**           | ALL (graduated controls)                                 |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai hệ thống chống brute-force attack toàn diện: progressive rate limiting cho login attempts, account lockout policy, CAPTCHA integration, credential stuffing detection, và notification cho user khi có suspicious login.

### 1.2 Bối cảnh
Bytebase hỗ trợ 2FA (SEC-12) và password policy (SEC-13) nhưng thiếu: login rate limiting, progressive lockout, CAPTCHA, credential stuffing detection, suspicious login alerts.

---

## 2. Yêu cầu chức năng

### FR-001: Progressive Login Rate Limiting
- **Policy**:
  ```yaml
  rate_limiting:
    login:
      per_account:
        - attempts: 3, action: "delay_2s"
        - attempts: 5, action: "captcha"
        - attempts: 10, action: "lockout_15m"
        - attempts: 20, action: "lockout_1h"
        - attempts: 50, action: "lockout_24h_notify_admin"
      per_ip:
        - attempts: 20/min, action: "captcha"
        - attempts: 50/min, action: "block_ip_1h"
      global:
        - attempts: 1000/min, action: "enable_global_captcha"
  ```
- **Acceptance Criteria**:
  - AC-1: Per-account progressive lockout (delay → captcha → lockout)
  - AC-2: Per-IP rate limiting (independent of account)
  - AC-3: Global rate limiting (DDoS protection on login endpoint)
  - AC-4: Lockout duration resets after successful login
  - AC-5: Admin can unlock any account immediately

### FR-002: Account Lockout Policy
- Configurable lockout thresholds, duration, và notification
- **Acceptance Criteria**:
  - AC-1: Admin configurable max failed attempts (default 10)
  - AC-2: Lockout duration escalation (15m → 1h → 24h)
  - AC-3: User email notification on lockout
  - AC-4: Admin notification on repeated lockouts
  - AC-5: Self-service unlock via email verification link
  - AC-6: Lockout bypass for SSO login (SSO has own protection)

### FR-003: CAPTCHA Integration
- Support reCAPTCHA v3 / hCaptcha / Turnstile
- **Acceptance Criteria**:
  - AC-1: Trigger CAPTCHA after N failed attempts (configurable)
  - AC-2: Support multiple CAPTCHA providers
  - AC-3: CAPTCHA bypass for API key / Service Account auth
  - AC-4: Invisible CAPTCHA mode (score-based, no user interaction)

### FR-004: Suspicious Login Detection & Notification
- Detect logins from new devices/locations/IPs
- **Acceptance Criteria**:
  - AC-1: Email alert on login from new device
  - AC-2: Email alert on login from new location (GeoIP)
  - AC-3: "Was this you?" confirmation mechanism
  - AC-4: Login history page for user review
  - AC-5: Admin view of all suspicious login events

### FR-005: Credential Stuffing Detection
- Detect automated credential testing patterns
- **Acceptance Criteria**:
  - AC-1: Detect rapid sequential login attempts across multiple accounts
  - AC-2: Block IPs with distributed attack pattern
  - AC-3: Integration with known breach databases (HIBP API) optional
  - AC-4: Alert admin on detected credential stuffing attack

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Rate Limiter (new)           | `backend/component/ratelimit/`              | Sliding window rate limiter                 |
| Auth Service                 | `backend/api/v1/auth_service.go`            | Login attempt tracking, lockout logic       |
| CAPTCHA Service (new)        | `backend/component/captcha/`                | Multi-provider CAPTCHA validation           |
| Login History Store (new)    | `backend/store/login_history.go`            | Login event persistence                     |
| GeoIP Service (new)          | `backend/component/geoip/`                  | IP to location resolution                   |
| Notification Service         | `backend/plugin/webhook/`                   | Suspicious login alerts                     |
| Login History UI (new)       | `frontend/src/views/LoginHistory.vue`       | Login history + device management           |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | 5 failed login attempts                              | CAPTCHA triggered                |
| TC-002  | 10 failed login attempts                             | Account locked 15m               |
| TC-003  | Login from new country                               | Email alert sent                 |
| TC-004  | 50 accounts targeted from 1 IP in 1 minute           | IP blocked                       |
| TC-005  | Admin unlocks locked account                         | Account immediately accessible   |
| TC-006  | Self-service unlock via email                        | Account unlocked after verify    |
| TC-007  | SSO login while account locked (password)            | SSO login succeeds               |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Progressive rate limiting            | Sprint 1       |
| Phase 2 | Account lockout + notification       | Sprint 1       |
| Phase 3 | CAPTCHA integration                  | Sprint 2       |
| Phase 4 | Suspicious login detection           | Sprint 3       |
| Phase 5 | Credential stuffing detection        | Sprint 4       |
