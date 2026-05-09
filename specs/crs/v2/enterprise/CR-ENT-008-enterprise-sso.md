# Change Request: Enterprise SSO (OIDC/SAML/LDAP)

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-008                                               |
| **Feature ID**     | SEC-11                                                   |
| **Title**          | Enterprise SSO — OIDC, SAML, LDAP                       |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai full Enterprise SSO support bao gồm **OIDC (OpenID Connect)**, **SAML 2.0**, và **LDAP** identity providers. Khác biệt với TEAM plan chỉ hỗ trợ Google/GitHub OAuth2 (SEC-05).

### 1.2 Bối cảnh
| Plan       | SSO                              |
|------------|----------------------------------|
| FREE       | —                                |
| TEAM       | Google / GitHub (OAuth2)         |
| ENTERPRISE | Full (OIDC / SAML / LDAP)       |

### 1.3 Mục tiêu
- Tích hợp với enterprise identity providers (Azure AD, Okta, OneLogin, Keycloak, etc.)
- Support multi-IdP configuration (multiple SSO providers simultaneously)
- Auto-provisioning users from IdP
- JIT (Just-In-Time) user creation trên first SSO login

---

## 2. Yêu cầu chức năng

### FR-001: OIDC Integration
- **Mô tả**: Support OpenID Connect 1.0 protocol.
- **Supported IdPs**: Azure AD, Okta, Keycloak, Auth0, Google Workspace, etc.
- **Configuration**:
  ```yaml
  oidc:
    issuer: "https://login.microsoftonline.com/{tenant}/v2.0"
    client_id: "xxx"
    client_secret: "xxx"
    scopes: ["openid", "profile", "email", "groups"]
    field_mapping:
      identifier: "email"
      display_name: "name"
      phone: "phone_number"
      groups: "groups"
  ```
- **Acceptance Criteria**:
  - AC-1: Support OIDC Discovery (`.well-known/openid-configuration`)
  - AC-2: Support Authorization Code Flow (PKCE optional)
  - AC-3: Token validation (signature, expiry, audience, issuer)
  - AC-4: Custom field mapping for user attributes
  - AC-5: Group membership sync from IdP claims

### FR-002: SAML 2.0 Integration
- **Mô tả**: Support SAML 2.0 Service Provider (SP) role.
- **Configuration**:
  ```yaml
  saml:
    metadata_url: "https://idp.example.com/metadata"
    # OR
    sso_url: "https://idp.example.com/sso"
    certificate: "-----BEGIN CERTIFICATE-----..."
    entity_id: "bytebase-sp"
    name_id_format: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
    attribute_mapping:
      email: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
      display_name: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
  ```
- **Acceptance Criteria**:
  - AC-1: SP metadata endpoint (`/saml/metadata`)
  - AC-2: SP-initiated SSO flow
  - AC-3: IdP-initiated SSO flow
  - AC-4: SAML assertion validation (signature, conditions, audience)
  - AC-5: Support signed assertions và signed responses

### FR-003: LDAP Integration
- **Mô tả**: Support LDAP/Active Directory authentication.
- **Configuration**:
  ```yaml
  ldap:
    host: "ldap.example.com"
    port: 636
    use_ssl: true
    bind_dn: "cn=admin,dc=example,dc=com"
    bind_password: "xxx"
    base_dn: "dc=example,dc=com"
    user_filter: "(objectClass=person)"
    field_mapping:
      identifier: "mail"
      display_name: "displayName"
      phone: "telephoneNumber"
      uid: "uid"
    group:
      base_dn: "ou=groups,dc=example,dc=com"
      filter: "(objectClass=groupOfNames)"
      member_attribute: "member"
  ```
- **Acceptance Criteria**:
  - AC-1: LDAP bind authentication (simple bind)
  - AC-2: LDAPS (LDAP over SSL/TLS) support
  - AC-3: StartTLS support
  - AC-4: User search và attribute mapping
  - AC-5: Group membership sync
  - AC-6: Connection test button trong config UI

### FR-004: Multi-IdP Configuration
- **Mô tả**: Support multiple SSO providers simultaneously.
- **Acceptance Criteria**:
  - AC-1: Login page hiển thị tất cả configured SSO buttons
  - AC-2: Mỗi IdP có unique identifier và display name
  - AC-3: User có thể link multiple IdPs tới cùng account
  - AC-4: Domain-based auto-redirect (e.g., @company.com → specific IdP)

### FR-005: User Auto-Provisioning
- **Mô tả**: Tự động tạo user account khi first-time SSO login.
- **Acceptance Criteria**:
  - AC-1: JIT user creation với attributes from IdP
  - AC-2: Default role assignment configurable per IdP
  - AC-3: Group mapping: IdP groups → Bytebase groups
  - AC-4: Seat limit check trước auto-provision (CR-ENT-002)
  - AC-5: Option để disable auto-provisioning (require pre-created account)

### FR-006: SSO Enforcement
- **Mô tả**: Admin có thể enforce SSO-only login.
- **Acceptance Criteria**:
  - AC-1: Option to disable password login (SSO only)
  - AC-2: Emergency admin bypass (recovery mode)
  - AC-3: API key / Service Account bypass SSO enforcement

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                         | Thay đổi                                          |
|------------------------------|--------------------------------------|----------------------------------------------------|
| OIDC Plugin                  | `backend/plugin/idp/oidc/`           | Full OIDC implementation (already exists, enhance) |
| SAML Plugin                  | `backend/plugin/idp/saml/`           | Full SAML SP (already exists, enhance)             |
| LDAP Plugin                  | `backend/plugin/idp/ldap/`           | LDAP auth + group sync (already exists, enhance)   |
| Auth Service                 | `backend/api/auth/`                  | Multi-IdP login flow                               |
| IdP Service (gRPC)           | `backend/api/v1/idp_service.go`      | CRUD identity providers                            |
| Feature Gate                 | `enterprise/feature.go`              | Define `FeatureEnterpriseSSO`                      |
| User Service                 | `backend/api/v1/user_service.go`     | Auto-provisioning logic                            |

### 3.2 Frontend Changes

| Component             | File                                        | Thay đổi                                    |
|-----------------------|---------------------------------------------|----------------------------------------------|
| Login Page            | `frontend/src/views/Login.vue`              | Multi-IdP SSO buttons                       |
| SSO Settings          | `frontend/src/views/SSOSettings.vue`         | IdP configuration UI                        |
| OIDC Config           | `frontend/src/components/OIDCConfig.vue`     | OIDC provider setup form                    |
| SAML Config           | `frontend/src/components/SAMLConfig.vue`     | SAML provider setup form                    |
| LDAP Config           | `frontend/src/components/LDAPConfig.vue`     | LDAP provider setup form + test             |

---

## 4. Security Considerations

| Concern                | Mitigation                                                    |
|------------------------|---------------------------------------------------------------|
| Token replay           | Nonce validation for OIDC, assertion replay detection SAML    |
| MITM attacks           | Enforce HTTPS for all SSO redirects, LDAPS/StartTLS           |
| Account takeover       | Validate email domain, require email verification             |
| IdP compromise         | Certificate pinning for SAML, JWKS validation for OIDC       |
| Credential storage     | Store client secrets in External Secret Manager (CR-ENT-015)  |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | OIDC login via Azure AD                                 | User authenticated, session created   |
| TC-002     | SAML SP-initiated login via Okta                        | SSO redirect → assertion → login      |
| TC-003     | SAML IdP-initiated login                                | Assertion received → user login       |
| TC-004     | LDAP bind authentication                                | User authenticated via LDAP           |
| TC-005     | First-time SSO login → auto-provision                   | User created with IdP attributes      |
| TC-006     | IdP group → Bytebase group mapping                      | Groups synced correctly               |
| TC-007     | SSO enforcement: password login blocked                 | Login form hidden, SSO only           |
| TC-008     | Multiple IdPs on login page                             | All configured IdPs shown             |
| TC-009     | Connection test for LDAP                                | Success/failure feedback               |
| TC-010     | Non-ENTERPRISE: OIDC/SAML config hidden                 | Feature gated, not accessible          |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | OIDC enhancement                     | Sprint 1       |
| Phase 2 | SAML enhancement                     | Sprint 2       |
| Phase 3 | LDAP enhancement                     | Sprint 2       |
| Phase 4 | Multi-IdP + auto-provision           | Sprint 3       |
| Phase 5 | SSO enforcement                      | Sprint 3       |
| Phase 6 | Security hardening + testing         | Sprint 4       |
