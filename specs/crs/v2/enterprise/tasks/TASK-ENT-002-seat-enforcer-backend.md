# TASK-ENT-002 — SeatEnforcer Component & Backend Enforcement

| Field            | Value                                      |
|------------------|--------------------------------------------|
| **Task ID**      | TASK-ENT-002                               |
| **Source**       | SOL-ENT-002 (CR-ENT-002)                  |
| **Status**       | Done                                       |
| **Priority**     | P0                                         |
| **Complexity**   | Medium                                     |
| **Sprint**       | Sprint 1                                   |

---

## Mô tả

Triển khai `SeatEnforcer` component (L5) tập trung seat check logic và integrate vào tất cả user creation channels.

## Scope

### Backend
1. **L9 — Enterprise**: Implement `GetSeatLimit()` — TEAM/ENTERPRISE = unlimited (-1), FREE = 20
2. **L8 — Store**: Implement `CountActiveUsers(ctx)` — exclude `SERVICE_ACCOUNT` và `WORKLOAD_IDENTITY`
3. **L5 — SeatEnforcer (NEW)**: `component/seat/enforcer.go` — centralized `CheckSeatAvailability()`
4. **L4 — Integration**: Integrate SeatEnforcer vào:
   - `user_service.go` → `CreateUser()`, `InviteUser()`
   - `auth/auth.go` → JIT SSO user creation
   - `directorysync/scim.go` → SCIM `POST /Users`
   - `oauth2/` → first-time login
5. **Advisory lock**: Xử lý race condition concurrent invites

### Frontend
6. **Members.vue**: Hiển thị `{current}/{max}` cho FREE, `{current} seats` cho TEAM/ENTERPRISE
7. **InviteDialog.vue**: Disable invite button + tooltip khi đạt limit

## Acceptance Criteria

- [x] `SeatEnforcer.CheckSeatAvailability()` hoạt động chính xác
- [x] `CountActiveUsers()` exclude service accounts và workload identities
- [x] Tất cả 5 user creation channels đều gọi SeatEnforcer
- [x] ENTERPRISE/TEAM plan tạo unlimited users
- [x] FREE plan block tại limit 20
- [x] Frontend disable invite khi đạt limit
- [x] Unit tests cho SeatEnforcer, CountActiveUsers
- [x] Downgrade plan: giữ users, block tạo mới

## Dependencies

- SOL-ENT-008 (SSO) — SSO auto-provisioning cần check seats
- SOL-ENT-014 (SCIM) — SCIM phải respect seat limits

## Definition of Done

- [x] SeatEnforcer unit tested
- [x] All 5 channels integrated & tested
- [x] Frontend quota display functional
