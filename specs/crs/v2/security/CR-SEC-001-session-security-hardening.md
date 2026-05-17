# Change Request: Session Security Hardening

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-001                                               |
| **Feature ID**     | SEC-01, SEC-12                                           |
| **Title**          | Session Security Hardening                               |
| **Plan**           | ALL (graduated controls)                                 |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Tăng cường bảo mật session toàn diện: JWT token lifecycle hardening, cookie hardening, concurrent session control, session fingerprinting, idle timeout, và token blacklisting.

### 1.2 Bối cảnh
Bytebase sử dụng JWT + Cookie (NF-SE04) nhưng thiếu: session fingerprinting, concurrent session limits, real-time token revocation, refresh token rotation enforcement, cookie attribute hardening.

### 1.3 Mục tiêu
- Zero-trust session management
- Token theft detection qua device/browser fingerprinting
- Concurrent session control
- Immediate token revocation via blacklisting
- Configurable idle timeout

---

## 2. Yêu cầu chức năng

### FR-001: JWT Token Lifecycle Hardening
- Access token: RS256, expiry ≤ 15m, JTI claim bắt buộc
- Refresh token: rotation on each refresh, reuse detection (→ revoke all tokens)
- Algorithm restriction: chỉ RS256/ES256

### FR-002: Cookie Security Hardening
- `HttpOnly`, `Secure`, `SameSite=Strict`, `__Host-` prefix (production)
- Cookie expiry đồng bộ với token expiry

### FR-003: Session Fingerprinting
- Hash User-Agent + Accept-Language + IP subnet + TLS characteristics
- Embed fingerprint trong JWT, validate mỗi request
- Mismatch → invalidate token + security alert
- Configurable strictness: `strict`/`relaxed`/`off`

### FR-004: Concurrent Session Control
- Default max 5 concurrent sessions per user (configurable 1-20)
- On limit exceeded: `terminate_oldest` or `deny_new`
- User xem/terminate sessions từ profile; Service accounts exempt

### FR-005: Idle Session Timeout
- Default 30m idle timeout, warning 5m trước
- Activity detection reset timer
- SQL Editor long-running queries không trigger timeout

### FR-006: Token Blacklisting
- In-memory + persistent blacklist, propagation < 1s
- Bulk revocation by user/device
- Password change / account disable → auto-revoke all tokens

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Auth Service                 | `backend/api/v1/auth_service.go`            | Token lifecycle, rotation logic             |
| Auth Interceptor             | `backend/api/interceptor/auth.go`           | Fingerprint validation, blacklist check     |
| Session Store (new)          | `backend/store/session.go`                  | Session registry, concurrent tracking       |
| Token Blacklist (new)        | `backend/component/auth/blacklist.go`       | In-memory + persistent blacklist            |
| Cookie Middleware (new)      | `backend/api/middleware/cookie.go`          | Cookie attribute enforcement                |
| Frontend Session Manager     | `frontend/src/utils/session.ts`             | Idle detection, activity tracking           |
| Active Sessions UI (new)     | `frontend/src/views/ActiveSessions.vue`     | View/terminate sessions                     |

---

## 4. Security Considerations

| Concern                     | Mitigation                                                    |
|-----------------------------|---------------------------------------------------------------|
| Token theft via XSS         | HttpOnly cookies, CSP, token fingerprinting                   |
| Session fixation             | New session ID on login, rotate on privilege change            |
| Refresh token theft          | Rotation + reuse detection → auto-revoke family               |
| Session riding (CSRF)        | SameSite=Strict + CSRF token                                  |

---

## 5. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Access token expired → refresh                       | New token pair issued            |
| TC-002  | Reuse old refresh token                              | All user tokens revoked          |
| TC-003  | Login from 6th device (limit=5)                      | Oldest session terminated        |
| TC-004  | Idle 31m (timeout=30m)                               | Redirect to login                |
| TC-005  | Token used with different User-Agent                 | Token invalidated                |
| TC-006  | Admin terminates user session                        | User immediately logged out      |
| TC-007  | Password change                                      | All sessions terminated          |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Cookie hardening + token lifecycle   | Sprint 1       |
| Phase 2 | Session fingerprinting               | Sprint 1       |
| Phase 3 | Concurrent session control           | Sprint 2       |
| Phase 4 | Idle timeout + warning UI            | Sprint 2       |
| Phase 5 | Token blacklisting + sessions UI     | Sprint 3       |
