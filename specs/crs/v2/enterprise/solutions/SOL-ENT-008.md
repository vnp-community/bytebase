# Solution: CR-ENT-008 — Enterprise SSO (OIDC/SAML/LDAP)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-008                |
| **Solution**   | SOL-ENT-008               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Enhance existing IDP plugins (L7: `plugin/idp/oidc/`, `plugin/idp/saml/`, `plugin/idp/ldap/`) để đạt full enterprise-grade SSO. Thêm multi-IdP support, JIT auto-provisioning, group mapping, SSO enforcement, và domain-based auto-redirect.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L7 — Plugin** | `plugin/idp/oidc/` | OIDC 1.0 (Azure AD, Okta, Keycloak) |
| **L7 — Plugin** | `plugin/idp/saml/` | SAML 2.0 SP (SP-initiated + IdP-initiated) |
| **L7 — Plugin** | `plugin/idp/ldap/` | LDAP/AD bind + group sync |
| **L4 — Service** | `auth/` | Multi-IdP login flow |
| **L4 — Service** | `idp_service.go` | IdP configuration CRUD |
| **L4 — Service** | `user_service.go` | JIT auto-provisioning logic |
| **L8 — Store** | IdP config storage | Multi-IdP configurations |
| **L9 — Enterprise** | `feature.go` | `FeatureEnterpriseSSO` gate |
| **L1 — Presentation** | Login page, SSO settings | Multi-IdP buttons, config forms |

---

## 3. Chi tiết Implementation

### 3.1 Multi-IdP Architecture

```
Login Page → Display all configured IdP buttons
  → User clicks "Login with Azure AD"
    → Redirect to OIDC Authorization endpoint
    → Callback → Validate token (JWKS)
    → Extract user attributes (email, name, groups)
    → JIT provisioning (if new user + seat available)
    → Group mapping (IdP groups → Bytebase groups)
    → Create session
```

### 3.2 OIDC Enhancement

- OIDC Discovery support (`.well-known/openid-configuration`)
- Authorization Code Flow with PKCE
- Custom field mapping for user attributes
- Group claims extraction → Bytebase group sync

### 3.3 SAML Enhancement

- SP metadata endpoint (`/saml/metadata`)
- SP-initiated và IdP-initiated SSO flows
- Certificate validation và assertion signature verification
- SAML attribute mapping configurable

### 3.4 LDAP Enhancement

- LDAPS (TLS) và StartTLS support
- Group sync: `memberOf` attribute → Bytebase groups
- Connection test endpoint cho configuration validation

### 3.5 SSO Enforcement

```go
// Setting: SSO_ENFORCEMENT = "SSO_ONLY" | "OPTIONAL"
// When SSO_ONLY: password login disabled
// Bypass: Service Accounts, API keys, emergency admin recovery
```

### 3.6 JIT User Provisioning

```go
func (s *AuthService) handleSSOLogin(ctx context.Context, idpUser *IdPUser) error {
    existingUser, _ := s.store.GetUserByEmail(ctx, idpUser.Email)
    if existingUser == nil {
        // Check seat limit (CR-ENT-002)
        if err := s.seatEnforcer.CheckSeatAvailability(ctx); err != nil {
            return err
        }
        // Create user with IdP attributes
        s.store.CreateUser(ctx, &store.UserMessage{...})
    }
    // Sync group memberships
    s.syncGroupMemberships(ctx, idpUser)
    return nil
}
```

---

## 4. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Token replay | Nonce validation (OIDC), assertion replay detection (SAML) |
| MITM | Enforce HTTPS, LDAPS/StartTLS |
| IdP compromise | Certificate pinning (SAML), JWKS validation (OIDC) |
| Credential storage | Client secrets via External Secret Manager (CR-ENT-015) |

---

## 5. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-002 | Seat limit check on JIT provisioning |
| CR-ENT-014 | SCIM provides alternative provisioning channel |
| CR-ENT-015 | Client secrets stored in external SM |

---

## 6. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | OIDC enhancement | Sprint 1 |
| 2 | SAML enhancement | Sprint 2 |
| 3 | LDAP enhancement | Sprint 2 |
| 4 | Multi-IdP + auto-provision | Sprint 3 |
| 5 | SSO enforcement | Sprint 3 |
| 6 | Security hardening + testing | Sprint 4 |
