# Change Request: Content Security Policy & HTTP Security Headers

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-015                                               |
| **Feature ID**     | NF-SE05                                                  |
| **Title**          | Content Security Policy & HTTP Security Headers          |
| **Plan**           | ALL                                                      |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai comprehensive HTTP security headers bao gồm CSP (Content Security Policy), CORS hardening, và các defense-in-depth headers. Bytebase đã có CSP cơ bản (NF-SE05: `vite-plugin-export-csp-hashes.ts`) nhưng cần hardening toàn diện.

---

## 2. Yêu cầu chức năng

### FR-001: Content Security Policy (CSP)
- **Policy**:
  ```
  Content-Security-Policy:
    default-src 'self';
    script-src 'self' 'nonce-{random}';
    style-src 'self' 'unsafe-inline';
    img-src 'self' data: blob:;
    font-src 'self';
    connect-src 'self' wss: https://api.bytebase.com;
    frame-ancestors 'none';
    base-uri 'self';
    form-action 'self';
    object-src 'none';
    upgrade-insecure-requests;
  ```
- **Acceptance Criteria**:
  - AC-1: Nonce-based script loading (no `unsafe-eval`)
  - AC-2: CSP violation reporting endpoint
  - AC-3: Report-only mode for testing before enforcement
  - AC-4: Monaco Editor compatible CSP (may need worker-src)
  - AC-5: SSO redirect domains configurable in `connect-src`

### FR-002: HTTP Security Headers Suite
- **Headers**:

| Header                        | Value                              | Purpose                          |
|-------------------------------|------------------------------------|----------------------------------|
| `X-Content-Type-Options`      | `nosniff`                          | Prevent MIME sniffing            |
| `X-Frame-Options`             | `DENY`                             | Prevent clickjacking             |
| `X-XSS-Protection`           | `0`                                | Disable legacy XSS filter        |
| `Referrer-Policy`             | `strict-origin-when-cross-origin`  | Control referrer leakage         |
| `Permissions-Policy`          | `camera=(), microphone=(), geolocation=()` | Restrict browser APIs |
| `Cross-Origin-Embedder-Policy`| `require-corp`                     | Cross-origin isolation           |
| `Cross-Origin-Opener-Policy`  | `same-origin`                      | Cross-origin isolation           |
| `Cross-Origin-Resource-Policy`| `same-origin`                      | Resource isolation               |

- **Acceptance Criteria**:
  - AC-1: All headers set on every response
  - AC-2: Headers configurable per environment
  - AC-3: No header conflicts with SSO/OAuth flows
  - AC-4: Security header score A+ on securityheaders.com

### FR-003: CORS Hardening
- **Acceptance Criteria**:
  - AC-1: Strict origin allowlist (no wildcard `*`)
  - AC-2: Credentials allowed only for specified origins
  - AC-3: Allowed methods restricted to actually used HTTP methods
  - AC-4: Max-Age for preflight caching
  - AC-5: Custom headers explicitly listed

### FR-004: Subresource Integrity (SRI)
- **Acceptance Criteria**:
  - AC-1: SRI hashes for all external script/style resources
  - AC-2: Build pipeline auto-generates SRI hashes
  - AC-3: SRI violation reported via CSP reporting endpoint

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| Security Headers Middleware  | `backend/api/middleware/security_headers.go` | All HTTP security headers                  |
| CSP Generator                | `frontend/vite-plugin-export-csp-hashes.ts`  | Enhanced CSP with nonce                    |
| CORS Config                  | `backend/server/server.go`                   | Strict CORS policy                         |
| CSP Report Endpoint (new)    | `backend/api/v1/csp_report.go`              | CSP violation reporting                    |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Inline script injection attempt                      | Blocked by CSP                   |
| TC-002  | Clickjacking via iframe                              | Blocked by X-Frame-Options       |
| TC-003  | Cross-origin request from non-allowed domain         | CORS blocked                     |
| TC-004  | CSP violation report                                 | Report received at endpoint      |
| TC-005  | Monaco Editor functionality with CSP                 | Editor works correctly           |
| TC-006  | SSO redirect with strict CSP                         | Redirect succeeds                |
| TC-007  | Security headers scan (securityheaders.com)          | Score A+                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | HTTP security headers suite          | Sprint 1       |
| Phase 2 | CSP enhancement (report-only mode)   | Sprint 1       |
| Phase 3 | CSP enforcement + CORS hardening     | Sprint 2       |
| Phase 4 | SRI + CSP reporting dashboard        | Sprint 3       |
