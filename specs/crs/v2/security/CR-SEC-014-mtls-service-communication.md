# Change Request: Mutual TLS (mTLS) for Service Communication

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-SEC-014                                               |
| **Feature ID**     | SEC-02                                                   |
| **Title**          | Mutual TLS (mTLS) for Service Communication             |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-13                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai Mutual TLS (mTLS) cho tất cả communications: Bytebase → database instances, Bytebase → external services (Vault, SIEM, IdP), và client → Bytebase API. Bao gồm certificate management, rotation, CA trust chain, và certificate pinning.

### 1.2 Bối cảnh
Bytebase hỗ trợ Instance SSL (SEC-02) và SSH Tunnel (SEC-03), nhưng thiếu: mTLS enforcement, certificate lifecycle management, automated cert rotation, và cert-based authentication.

---

## 2. Yêu cầu chức năng

### FR-001: mTLS for Database Connections
- **Acceptance Criteria**:
  - AC-1: Client certificate authentication per database instance
  - AC-2: Support custom CA for self-signed certificates
  - AC-3: Certificate chain validation
  - AC-4: Per-engine TLS configuration (PostgreSQL, MySQL differ)
  - AC-5: TLS version enforcement (minimum TLS 1.2, prefer 1.3)
  - AC-6: Cipher suite restriction (AEAD ciphers only)

### FR-002: Certificate Lifecycle Management
- **Acceptance Criteria**:
  - AC-1: Internal CA for self-signed certificate generation
  - AC-2: Certificate expiry monitoring with alerts
  - AC-3: Automated certificate renewal (Let's Encrypt, ACME)
  - AC-4: Certificate rotation without service restart
  - AC-5: Certificate inventory dashboard

### FR-003: Client Certificate Authentication
- Alternative to API key/JWT for machine-to-machine auth
- **Acceptance Criteria**:
  - AC-1: Client certificate as authentication method for API
  - AC-2: Certificate-to-service-account mapping
  - AC-3: Certificate revocation list (CRL) support
  - AC-4: OCSP stapling support

### FR-004: TLS Configuration Hardening
- **Acceptance Criteria**:
  - AC-1: TLS 1.3 preferred, TLS 1.2 minimum
  - AC-2: HSTS header with preload readiness
  - AC-3: OCSP stapling enabled
  - AC-4: Perfect Forward Secrecy (PFS) enforced
  - AC-5: TLS configuration scoring (A+ target on SSL Labs)

---

## 3. Yêu cầu kỹ thuật

| Component                    | File/Package                                | Thay đổi                                   |
|------------------------------|---------------------------------------------|---------------------------------------------|
| TLS Manager (new)            | `backend/component/tls/`                    | Certificate management, rotation            |
| DB Plugins                   | `backend/plugin/db/*/`                      | mTLS per engine configuration               |
| Auth Interceptor             | `backend/api/interceptor/auth.go`           | Client certificate authentication           |
| Server Config                | `backend/server/server.go`                  | TLS hardening configuration                 |
| Certificate Dashboard        | `frontend/src/views/CertificateManager.vue` | Cert inventory + expiry tracking            |

---

## 4. Test Cases

| Test ID | Mô tả                                               | Expected Result                  |
|---------|------------------------------------------------------|----------------------------------|
| TC-001  | Connect to PostgreSQL with mTLS                      | Connection established           |
| TC-002  | Expired client certificate                           | Connection rejected              |
| TC-003  | TLS 1.1 connection attempt                           | Rejected (min TLS 1.2)          |
| TC-004  | Certificate auto-renewal                             | New cert active, no downtime     |
| TC-005  | Client cert authentication for API                   | Authenticated via certificate    |
| TC-006  | Certificate near expiry                              | Alert generated                  |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | TLS hardening + cipher config        | Sprint 1       |
| Phase 2 | mTLS for database connections        | Sprint 2       |
| Phase 3 | Certificate lifecycle management     | Sprint 3       |
| Phase 4 | Client cert authentication           | Sprint 4       |
