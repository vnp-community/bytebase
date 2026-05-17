# Solution: CR-ENT-002 — Maximum Seats (Unlimited)

| Field          | Value                     |
|----------------|---------------------------|
| **CR**         | CR-ENT-002                |
| **Solution**   | SOL-ENT-002               |
| **Status**     | Proposed                  |
| **Complexity** | Medium                    |

---

## 1. Tóm tắt giải pháp

Enforce seat limit tại **tất cả user creation channels** (API, SSO, SCIM, OAuth2) bằng cách tập trung logic vào `SeatEnforcer` component. TEAM/ENTERPRISE = unlimited, FREE = 20. Service Accounts không tính vào seat count.

---

## 2. Architectural Alignment

```
L1 Frontend ──► L2 API Gateway ──► L3 Auth Interceptor
                                         │
                    ┌────────────────────┘
                    ▼
            L4 Service Layer
            ├── UserService.CreateUser
            ├── AuthService (SSO auto-provision)
            └── DirectorySyncService (SCIM)
                    │
                    ▼
            L5 Component: SeatEnforcer (NEW)
                    │
              ┌─────┴──────┐
              ▼            ▼
        L8 Store      L9 Enterprise
   CountActiveUsers   GetSeatLimit
```

| Layer | Component | Vai trò |
|-------|-----------|---------|
| **L9 — Enterprise** | `license.go` | `GetSeatLimit()` — trả về limit theo plan |
| **L5 — Component** | `component/seat/enforcer.go` (NEW) | Centralized seat check, tránh duplicate logic |
| **L4 — Service** | `user_service.go`, `auth/`, `directorysync/` | Gọi SeatEnforcer trước khi tạo user |
| **L8 — Store** | `store/principal.go` | `CountActiveUsers(ctx)` — exclude service accounts |
| **L1 — Presentation** | `Members.vue`, `InviteDialog.vue` | Quota display + invite disable |

---

## 3. Chi tiết Implementation

### 3.1 L5 — SeatEnforcer Component (New)

**File**: `backend/component/seat/enforcer.go`

```go
type SeatEnforcer struct {
    store          *store.Store
    licenseService enterprise.LicenseService
}

// CheckSeatAvailability returns nil if seat is available, error otherwise.
func (e *SeatEnforcer) CheckSeatAvailability(ctx context.Context) error {
    limit, _ := e.licenseService.GetSeatLimit(ctx)
    if limit < 0 { // unlimited
        return nil
    }

    count, err := e.store.CountActiveUsers(ctx)
    if err != nil {
        return err
    }

    if count >= limit {
        return status.Errorf(codes.ResourceExhausted,
            "seat limit reached (%d/%d). Upgrade plan for more seats.", count, limit)
    }
    return nil
}
```

**Rationale**: Tập trung seat check vào 1 component thay vì duplicate logic ở 5+ channels (API, SSO, SCIM, OAuth2, invitation).

### 3.2 L8 — Store Layer

**File**: `backend/store/principal.go`

```go
func (s *Store) CountActiveUsers(ctx context.Context) (int, error) {
    query := `SELECT COUNT(*) FROM principal
              WHERE row_status = 'NORMAL'
              AND type != 'SERVICE_ACCOUNT'
              AND type != 'WORKLOAD_IDENTITY'`
    var count int
    err := s.dbConnManager.GetDB().QueryRowContext(ctx, query).Scan(&count)
    return count, err
}
```

### 3.3 L4 — Service Layer Integration Points

| Channel | File | Integration |
|---------|------|-------------|
| Direct creation | `user_service.go` → `CreateUser()` | Call `seatEnforcer.CheckSeatAvailability()` |
| Email invitation | `user_service.go` → `InviteUser()` | Call before sending invite |
| SSO auto-provision | `auth/auth.go` → JIT user creation | Call during SSO login flow |
| SCIM sync | `directorysync/scim.go` → `POST /Users` | Skip check cho ENTERPRISE (unlimited) |
| OAuth2 first login | `oauth2/` → first-time login | Call before user creation |

### 3.4 L1 — Frontend

- **Members.vue**: Hiển thị `{current}/{max}` cho FREE, `{current} seats` cho TEAM/ENTERPRISE.
- **InviteDialog.vue**: Disable invite button + tooltip khi đạt limit.
- **Settings.vue**: Plan limits overview với upgrade CTA.

---

## 4. Database Changes

**Không cần migration.** Sử dụng `COUNT(*)` trên bảng `principal` hiện có.

---

## 5. Billing Integration (Stripe)

```go
// Report seat count to Stripe for usage-based billing
func (s *SubscriptionService) ReportSeatUsage(ctx context.Context) error {
    count, _ := s.store.CountActiveUsers(ctx)
    // Update Stripe subscription usage record
    return s.stripeClient.UpdateUsageRecord(ctx, count)
}
```

Trigger: Chạy sau mỗi user create/deactivate event.

---

## 6. Phụ thuộc & Rủi ro

| Phụ thuộc | CR |
|-----------|-----|
| SCIM/Directory Sync | CR-ENT-014 — SCIM phải respect seat limits |
| Enterprise SSO | CR-ENT-008 — SSO auto-provisioning cần check |
| Billing | Stripe subscription phải phản ánh seat count |

| Rủi ro | Mitigation |
|--------|-----------|
| SCIM bulk sync vượt limit | ENTERPRISE = unlimited, nên không vấn đề |
| Race condition concurrent invites | Advisory lock trên seat count check |
| Downgrade plan | Giữ users, block tạo mới |

---

## 7. Kế hoạch triển khai

| Phase | Scope | Sprint |
|-------|-------|--------|
| 1 | SeatEnforcer + backend enforcement | Sprint 1 |
| 2 | Frontend quota display | Sprint 1 |
| 3 | SSO/SCIM integration | Sprint 2 |
| 4 | Stripe billing sync | Sprint 3 |
| 5 | E2E testing | Sprint 3 |
