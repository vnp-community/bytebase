# Solution: CR-ENT-014 — SCIM / Directory Sync

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-014                |
| **Solution**   | SOL-ENT-014               |
| **Status**     | Proposed                  |
| **Complexity** | High                      |

---

## 1. Tóm tắt giải pháp

Enhance SCIM service hiện có (`backend/api/v1/directorysync/`) để đạt full SCIM 2.0 compliance (RFC 7643/7644). Service đã tồn tại với basic endpoints — cần mở rộng với group sync, SCIM filter support, pagination, và token management.

---

## 2. Architectural Alignment

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L1 — Protocol Adapter** | `backend/api/directory-sync/` | SCIM 2.0 REST endpoints (existing) |
| **L4 — Service** | `user_service.go` | SCIM-triggered user provisioning |
| **L4 — Service** | `group_service.go` | SCIM-triggered group sync |
| **L3 — Security** | `auth/` | SCIM bearer token authentication |
| **L8 — Store** | User/Group stores | Persistence |
| **L9 — Enterprise** | `feature.go` | `FeatureSCIMDirectorySync` gate |

---

## 3. Chi tiết Implementation

### 3.1 SCIM Endpoints (RFC 7644)

Already routed via Echo: `/hook/scim/*` (architecture.md §3 Route Map).

**Enhance**:
- `GET /scim/v2/Users?filter=userName eq "..."` — SCIM filter support
- `PATCH /scim/v2/Users/{id}` — Partial update (RFC 7644 §3.5.2)
- Full Group endpoints (create, update, delete)
- `/scim/v2/ServiceProviderConfig` — Capability discovery
- `/scim/v2/Schemas` — Schema discovery

### 3.2 User Provisioning Logic

```go
func (s *SCIMService) CreateUser(ctx context.Context, scimUser *SCIMUser) error {
    // ENTERPRISE = unlimited seats → skip seat check
    // Map SCIM user schema → Bytebase UserMessage
    user := &store.UserMessage{
        Email:       scimUser.Emails[0].Value,
        Name:        scimUser.DisplayName(),
        Type:        store.EndUser,
    }
    return s.store.CreateUser(ctx, user)
}

func (s *SCIMService) DeleteUser(ctx context.Context, id string) error {
    // SCIM DELETE = deactivate (not hard delete)
    return s.store.UpdateUser(ctx, id, &store.UpdateUserMessage{RowStatus: "ARCHIVED"})
}
```

### 3.3 SCIM Token Management

Dedicated SCIM bearer tokens, separate from user API keys:
- Admin generates SCIM token via Settings UI
- Token rotatable without disrupting active sync
- Multiple active tokens for rotation window

---

## 4. Phụ thuộc

| CR | Relationship |
|----|-------------|
| CR-ENT-002 | ENTERPRISE unlimited seats → SCIM never blocked |
| CR-ENT-008 | IdP groups synced via SCIM complement SSO |

---

## 5. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | SCIM User endpoints enhancement | Sprint 1 |
| 2 | SCIM Group endpoints | Sprint 2 |
| 3 | Token management | Sprint 2 |
| 4 | Azure AD / Okta integration testing | Sprint 3 |
| 5 | E2E testing + documentation | Sprint 3 |
