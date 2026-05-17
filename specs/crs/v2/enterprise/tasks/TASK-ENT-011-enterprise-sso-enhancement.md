# TASK-ENT-011 — Enterprise SSO Enhancement (OIDC/SAML/LDAP)

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-011                               |
| **Source**       | SOL-ENT-008 (CR-ENT-008)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–4                                 |

---

## Mô tả

Enhance existing IDP plugins (L7) để đạt full enterprise-grade SSO: multi-IdP support, JIT auto-provisioning, group mapping, SSO enforcement, domain-based auto-redirect.

## Scope

### Phase 1 — Sprint 1: OIDC Enhancement
1. **L7 — OIDC Plugin**: OIDC Discovery, Authorization Code Flow + PKCE, custom field mapping, group claims extraction
2. **Multi-IdP UI**: Display all configured IdP buttons on login page

### Phase 2 — Sprint 2: SAML + LDAP
3. **L7 — SAML Plugin**: SP metadata endpoint, SP-initiated + IdP-initiated flows, certificate validation, assertion signature verification, configurable attribute mapping
4. **L7 — LDAP Plugin**: LDAPS/StartTLS, group sync via `memberOf`, connection test endpoint

### Phase 3 — Sprint 3: Auto-Provision + SSO Enforcement
5. **JIT User Provisioning**: `handleSSOLogin()` — check seat limit (CR-ENT-002), create user with IdP attributes, sync group memberships
6. **SSO Enforcement**: `SSO_ENFORCEMENT` setting — `SSO_ONLY` disables password login; bypass for Service Accounts, API keys, emergency admin
7. **Multi-IdP Configuration**: Multiple IdP instances support

### Phase 4 — Sprint 4: Security Hardening
8. **Security**: Nonce validation (OIDC), replay detection (SAML), HTTPS enforcement, LDAPS/StartTLS, certificate pinning (SAML), JWKS validation (OIDC)
9. **Client secrets**: Via External Secret Manager (CR-ENT-015)

## Acceptance Criteria

- [x] OIDC Discovery + PKCE flow functional
- [x] SAML SP-initiated + IdP-initiated SSO working
- [x] LDAP bind + group sync operational
- [x] Multi-IdP: login page shows all configured IdP buttons
- [x] JIT provisioning: new SSO user auto-created (if seats available)
- [x] Group mapping: IdP groups → Bytebase groups synced
- [x] SSO enforcement: password login disabled when `SSO_ONLY`
- [x] Emergency admin bypass works when IdP is down
- [x] All security mitigations implemented

## Dependencies

- TASK-ENT-002 (Seat Limit) — seat check on JIT provisioning
- CR-ENT-014 (SCIM) — alternative provisioning channel
- CR-ENT-015 (External Secret Manager) — client secrets storage

## Definition of Done

- [x] All 3 IdP types (OIDC, SAML, LDAP) tested with real providers (Azure AD, Okta, Keycloak)
- [x] Security hardening verified
- [x] SSO enforcement E2E tested
