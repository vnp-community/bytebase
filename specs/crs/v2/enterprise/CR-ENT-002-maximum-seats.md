# Change Request: Maximum Seats — Unlimited

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-002                                               |
| **Feature ID**     | PRICING-SEATS                                            |
| **Title**          | Enterprise Maximum Seats — Unlimited                     |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Cho phép workspace ENTERPRISE có **không giới hạn** số lượng user seats (người dùng), vượt qua giới hạn 20 seats của plan FREE.

### 1.2 Bối cảnh
| Plan       | Maximum Seats |
|------------|---------------|
| FREE       | 20            |
| TEAM       | Unlimited     |
| ENTERPRISE | Unlimited     |

### 1.3 Mục tiêu
- Xóa bỏ giới hạn seat count cho ENTERPRISE license
- Đảm bảo feature gate kiểm tra đúng plan khi invite/create user
- Hỗ trợ quản lý seats hiệu quả với SCIM/Directory Sync (CR-ENT-014)

---

## 2. Yêu cầu chức năng

### FR-001: License-based Seat Limit Enforcement
- **Mô tả**: Hệ thống phải kiểm tra license plan trước khi cho phép thêm user mới.
- **Logic**:
  ```
  IF plan == ENTERPRISE OR plan == TEAM:
      limit = UNLIMITED (no check)
  ELSE: // FREE
      limit = 20
  ```
- **Acceptance Criteria**:
  - AC-1: ENTERPRISE workspace có thể invite > 20 users mà không bị block
  - AC-2: FREE workspace bị block khi đạt 20 users với thông báo lỗi rõ ràng
  - AC-3: Seat count chỉ tính user ở trạng thái ACTIVE (không tính DEACTIVATED)
  - AC-4: Service Accounts không tính vào seat count

### FR-002: Seat Quota Display
- **Mô tả**: UI phải hiển thị seat usage quota phù hợp với plan.
- **Logic**:
  - FREE: Hiển thị `{current}/{max}` (e.g., `15/20`)
  - TEAM/ENTERPRISE: Hiển thị `{current}` (không hiển thị limit)
- **Acceptance Criteria**:
  - AC-1: Settings → Members page hiển thị đúng quota
  - AC-2: Khi gần đạt limit (≥80%), hiển thị warning cho FREE plan

### FR-003: Invitation & User Creation Enforcement
- **Mô tả**: Các flow tạo user phải enforce seat limit.
- **Channels cần enforce**:
  - Direct user creation (Admin UI)
  - Email invitation
  - SSO auto-provisioning (OIDC/SAML/LDAP)
  - SCIM directory sync (auto-provisioning)
  - OAuth2 first-time login
- **Acceptance Criteria**:
  - AC-1: Tất cả channels đều kiểm tra seat limit trước khi tạo user
  - AC-2: SCIM sync cho ENTERPRISE không bao giờ bị block bởi seat limit
  - AC-3: Error response chứa upgrade guidance

### FR-004: Billing Integration
- **Mô tả**: Seat count phải được report cho billing system (Stripe).
- **Acceptance Criteria**:
  - AC-1: Stripe subscription phản ánh actual seat count
  - AC-2: Usage-based billing cập nhật khi seat count thay đổi

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                        | Thay đổi                                       |
|------------------------------|-------------------------------------|-------------------------------------------------|
| License Service              | `enterprise/`                       | Expose `GetSeatLimit()` method                  |
| User Service (gRPC)          | `backend/api/v1/user_service.go`    | Check limit trước khi `CreateUser`              |
| Auth Service                 | `backend/api/auth/`                 | Check limit khi SSO auto-provisioning            |
| Feature Gate                 | `enterprise/feature.go`             | Define `FeatureUnlimitedSeats`                  |
| Store Layer                  | `backend/store/user.go`             | Thêm `CountActiveUsers(ctx)` query              |
| SCIM Service                 | `backend/api/v1/directorysync/`     | Skip seat check cho ENTERPRISE                   |

### 3.2 Frontend Changes

| Component           | File                                       | Thay đổi                              |
|---------------------|--------------------------------------------|---------------------------------------|
| Members Page        | `frontend/src/views/Members.vue`           | Hiển thị seat quota                   |
| Invite Dialog       | `frontend/src/components/InviteDialog.vue`  | Disable invite khi đạt limit          |
| Settings Page       | `frontend/src/views/Settings.vue`          | Hiển thị plan limits overview         |

### 3.3 Database Changes
Không cần schema migration — sử dụng `COUNT(*)` trên bảng `principal`/`user` hiện có.

---

## 4. Phụ thuộc

| Dependency            | Mô tả                                                      |
|-----------------------|--------------------------------------------------------------|
| License Service       | Phải có `LicenseService` hoạt động để xác định plan          |
| SCIM Service          | CR-ENT-014 (SCIM/Directory Sync) phải respect seat limits    |
| SSO Authentication    | CR-ENT-008 (Enterprise SSO) auto-provisioning cần kiểm tra   |

---

## 5. Test Cases

| Test ID    | Mô tả                                                      | Expected Result                       |
|------------|--------------------------------------------------------------|---------------------------------------|
| TC-001     | FREE plan tạo user thứ 21                                   | Error: RESOURCE_EXHAUSTED             |
| TC-002     | ENTERPRISE plan tạo user thứ 21                             | Success                               |
| TC-003     | ENTERPRISE plan tạo user thứ 500                            | Success                               |
| TC-004     | SCIM sync provision 50 users cho ENTERPRISE                  | All 50 created successfully           |
| TC-005     | SSO auto-provision khi FREE plan đã đạt limit                | Error: seat limit reached             |
| TC-006     | Deactivate user → seat count giảm → có thể tạo user mới     | Success                               |
| TC-007     | Service Account không tính vào seat count                    | Count excludes service accounts       |
| TC-008     | Downgrade từ ENTERPRISE → FREE khi > 20 users                | Giữ nguyên users, block tạo thêm     |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline       |
|---------|--------------------------------------|----------------|
| Phase 1 | Backend seat enforcement             | Sprint 1       |
| Phase 2 | Frontend quota display               | Sprint 1       |
| Phase 3 | SSO/SCIM auto-provision integration  | Sprint 2       |
| Phase 4 | Billing integration (Stripe)         | Sprint 3       |
| Phase 5 | E2E testing + documentation          | Sprint 3       |
