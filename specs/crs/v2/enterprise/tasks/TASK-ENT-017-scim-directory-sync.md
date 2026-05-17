# TASK-ENT-017 — SCIM 2.0 Directory Sync Enhancement

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-017                               |
| **Source**       | SOL-ENT-014 (CR-ENT-014)                  |
| **Status**       | Done                                       |
| **Priority**     | P1                                         |
| **Complexity**   | High                                       |
| **Sprint**       | Sprint 1–3                                 |

---

## Mô tả

Enhance SCIM service hiện có (`backend/api/v1/directorysync/`) để đạt full SCIM 2.0 compliance (RFC 7643/7644) với group sync, filter support, và token management.

## Scope

### Phase 1 — Sprint 1: User Endpoints Enhancement
1. **SCIM Filter Support**: `GET /scim/v2/Users?filter=userName eq "..."` — RFC 7644 compliant
2. **PATCH Support**: `PATCH /scim/v2/Users/{id}` — partial update (RFC 7644 §3.5.2)
3. **User Provisioning**: Map SCIM schema → Bytebase UserMessage; ENTERPRISE = skip seat check
4. **Deactivation**: SCIM DELETE = deactivate (ARCHIVED), not hard delete

### Phase 2 — Sprint 2: Group Endpoints + Token Management
5. **Group CRUD**: Full `/scim/v2/Groups` endpoints (create, update, delete)
6. **Schema Discovery**: `/scim/v2/Schemas`, `/scim/v2/ServiceProviderConfig`
7. **Token Management**: Dedicated SCIM bearer tokens (admin generates, rotatable, multiple active)

### Phase 3 — Sprint 3: IdP Integration Testing
8. **Azure AD Integration**: End-to-end testing with Azure AD SCIM provisioning
9. **Okta Integration**: End-to-end testing with Okta SCIM
10. **Documentation**: SCIM setup guide for supported IdPs

## Acceptance Criteria

- [x] SCIM filter support (RFC 7644 compliant)
- [x] SCIM PATCH partial update works
- [x] User provisioning maps SCIM → Bytebase correctly
- [x] SCIM DELETE deactivates (not hard delete)
- [x] Group CRUD endpoints functional
- [x] ServiceProviderConfig + Schemas discovery endpoints
- [x] SCIM token management: generate, rotate, multiple active
- [x] Azure AD SCIM provisioning tested end-to-end
- [x] Okta SCIM provisioning tested end-to-end

## Dependencies

- CR-ENT-002 (Seats) — ENTERPRISE unlimited → SCIM never blocked
- CR-ENT-008 (SSO) — IdP groups synced via SCIM complement SSO

## Definition of Done

- [x] RFC 7643/7644 compliance verified
- [x] Azure AD + Okta integration tested
- [x] SCIM token management functional
