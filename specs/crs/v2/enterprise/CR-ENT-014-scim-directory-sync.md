# Change Request: SCIM / Directory Sync

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-014                                               |
| **Feature ID**     | SEC-17                                                   |
| **Title**          | SCIM 2.0 / Directory Sync                                |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **SCIM 2.0 (System for Cross-domain Identity Management)** cho phép đồng bộ user và group directory từ enterprise identity providers (Azure AD, Okta, OneLogin) tới Bytebase workspace.

### 1.2 Mục tiêu
- SCIM 2.0 compliant endpoints (RFC 7643, RFC 7644)
- Auto-provision/deprovision users from IdP
- Group sync (create, update, delete groups)
- Incremental sync (polling) và webhook-based push

---

## 2. Yêu cầu chức năng

### FR-001: SCIM 2.0 Endpoints
- **Mô tả**: Implement SCIM 2.0 service provider endpoints.
- **Required Endpoints**:

| Method   | Endpoint                     | Mô tả                       |
|----------|------------------------------|-------------------------------|
| GET      | `/scim/v2/Users`             | List users                   |
| POST     | `/scim/v2/Users`             | Create user                  |
| GET      | `/scim/v2/Users/{id}`        | Get user                     |
| PUT      | `/scim/v2/Users/{id}`        | Replace user                 |
| PATCH    | `/scim/v2/Users/{id}`        | Partial update user          |
| DELETE   | `/scim/v2/Users/{id}`        | Delete (deactivate) user     |
| GET      | `/scim/v2/Groups`            | List groups                  |
| POST     | `/scim/v2/Groups`            | Create group                 |
| GET      | `/scim/v2/Groups/{id}`       | Get group                    |
| PUT      | `/scim/v2/Groups/{id}`       | Replace group                |
| PATCH    | `/scim/v2/Groups/{id}`       | Partial update group         |
| DELETE   | `/scim/v2/Groups/{id}`       | Delete group                 |
| GET      | `/scim/v2/ServiceProviderConfig` | Service provider config  |
| GET      | `/scim/v2/Schemas`           | Schema discovery             |
| GET      | `/scim/v2/ResourceTypes`     | Resource types               |

- **Acceptance Criteria**:
  - AC-1: All endpoints conform to SCIM 2.0 spec (RFC 7644)
  - AC-2: SCIM filter support (`filter=userName eq "john@example.com"`)
  - AC-3: Pagination support (`startIndex`, `count`)
  - AC-4: SCIM error responses với correct schema
  - AC-5: Bearer token authentication for SCIM API

### FR-002: User Provisioning
- **Mô tả**: Auto-provision users từ IdP directory.
- **User Schema Mapping**:
  ```json
  {
    "schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
    "userName": "john@example.com",
    "name": {
      "givenName": "John",
      "familyName": "Doe"
    },
    "emails": [{"value": "john@example.com", "primary": true}],
    "active": true,
    "groups": [{"value": "group-id", "display": "Engineering"}]
  }
  ```
- **Acceptance Criteria**:
  - AC-1: User created in Bytebase when provisioned via SCIM
  - AC-2: User deactivated (not deleted) when removed via SCIM
  - AC-3: User attributes updated on SCIM PUT/PATCH
  - AC-4: Seat limit respected (ENTERPRISE = unlimited, CR-ENT-002)

### FR-003: Group Sync
- **Mô tả**: Sync groups và group membership từ IdP.
- **Acceptance Criteria**:
  - AC-1: IdP groups created as Bytebase User Groups
  - AC-2: Group membership changes reflected in Bytebase
  - AC-3: Group deletion deactivates group (preserves assignments for audit)
  - AC-4: Group → Project role mapping configurable

### FR-004: SCIM Token Management
- **Mô tả**: Generate và manage SCIM API tokens.
- **Acceptance Criteria**:
  - AC-1: Admin can generate SCIM bearer token
  - AC-2: Token rotatable without disrupting sync
  - AC-3: Token revocation immediate
  - AC-4: Multiple active tokens supported (rotation window)

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                           | Thay đổi                                          |
|------------------------------|----------------------------------------|----------------------------------------------------|
| SCIM Service                 | `backend/api/v1/directorysync/`        | SCIM 2.0 endpoints (already exists, enhance)       |
| User Service                 | `backend/api/v1/user_service.go`       | SCIM-triggered user provisioning                   |
| Group Service                | `backend/api/v1/group_service.go`      | SCIM-triggered group sync                          |
| Feature Gate                 | `enterprise/feature.go`               | Define `FeatureSCIMDirectorySync`                  |
| Auth Middleware              | `backend/api/auth/`                    | SCIM bearer token authentication                   |

---

## 4. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                       |
|------------|----------------------------------------------------------|---------------------------------------|
| TC-001     | POST /scim/v2/Users — create user                       | User provisioned in Bytebase          |
| TC-002     | DELETE /scim/v2/Users/{id} — remove user                | User deactivated, not deleted         |
| TC-003     | PATCH /scim/v2/Users/{id} — update attributes           | User attributes updated               |
| TC-004     | GET /scim/v2/Users?filter=userName eq "..."              | Filtered user list returned           |
| TC-005     | POST /scim/v2/Groups — create group                     | Group created in Bytebase             |
| TC-006     | SCIM sync 100 users (bulk)                              | All users provisioned                 |
| TC-007     | Invalid SCIM token                                      | 401 Unauthorized                      |
| TC-008     | Non-ENTERPRISE: SCIM endpoints return 403                | Feature gated                         |

---

## 5. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | SCIM User endpoints                  | Sprint 1       |
| Phase 2 | SCIM Group endpoints                 | Sprint 2       |
| Phase 3 | Token management                     | Sprint 2       |
| Phase 4 | Azure AD / Okta integration test     | Sprint 3       |
| Phase 5 | E2E testing + documentation          | Sprint 3       |
