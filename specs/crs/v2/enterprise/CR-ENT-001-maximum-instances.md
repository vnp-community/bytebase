# Change Request: Maximum Instances — Unlimited

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ENT-001                                               |
| **Feature ID**     | PRICING-INSTANCES                                        |
| **Title**          | Enterprise Maximum Instances — Unlimited                 |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Cho phép workspace ENTERPRISE đăng ký **không giới hạn** số lượng database instances, vượt qua giới hạn 10 instances của plan FREE và TEAM.

### 1.2 Bối cảnh
| Plan       | Maximum Instances |
|------------|-------------------|
| FREE       | 10                |
| TEAM       | 10                |
| ENTERPRISE | Unlimited         |

### 1.3 Mục tiêu
- Xóa bỏ giới hạn instance count cho ENTERPRISE license
- Đảm bảo feature gate kiểm tra đúng plan khi tạo/import instance
- Cung cấp thông tin usage rõ ràng trên UI (quota sử dụng vs giới hạn)

---

## 2. Yêu cầu chức năng

### FR-001: License-based Instance Limit Enforcement
- **Mô tả**: Hệ thống phải kiểm tra license plan trước khi cho phép tạo mới hoặc activate một instance.
- **Logic**:
  ```
  IF plan == ENTERPRISE:
      limit = UNLIMITED (no check)
  ELSE IF plan == TEAM:
      limit = 10
  ELSE: // FREE
      limit = 10
  ```
- **Acceptance Criteria**:
  - AC-1: ENTERPRISE workspace có thể tạo > 10 instances mà không bị block
  - AC-2: FREE/TEAM workspace bị block khi đạt 10 instances với thông báo lỗi rõ ràng
  - AC-3: Instance count chỉ tính các instance ở trạng thái ACTIVE (không tính DELETED/ARCHIVED)

### FR-002: Instance Quota Display
- **Mô tả**: UI phải hiển thị instance usage quota phù hợp với plan.
- **Logic**:
  - FREE/TEAM: Hiển thị `{current}/{max}` (e.g., `7/10`)
  - ENTERPRISE: Hiển thị `{current}` (không hiển thị limit, hoặc hiển thị `∞`)
- **Acceptance Criteria**:
  - AC-1: Dashboard Settings hiển thị đúng quota theo plan
  - AC-2: Khi gần đạt limit (≥80%), hiển thị warning cho FREE/TEAM

### FR-003: API Enforcement
- **Mô tả**: API `CreateInstance`, `ActivateInstance` phải enforce instance limit.
- **Acceptance Criteria**:
  - AC-1: API trả về `RESOURCE_EXHAUSTED` (code 429) khi vượt limit
  - AC-2: Error message chứa thông tin plan hiện tại và hướng dẫn upgrade
  - AC-3: Terraform Provider phải handle error code này gracefully

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component                    | File/Package                        | Thay đổi                                      |
|------------------------------|-------------------------------------|------------------------------------------------|
| License Service              | `enterprise/`                       | Expose `GetInstanceLimit()` method             |
| Instance Service (gRPC)      | `backend/api/v1/instance_service.go`| Check limit trước khi `CreateInstance`         |
| Feature Gate                 | `enterprise/feature.go`             | Define `FeatureUnlimitedInstances`             |
| Store Layer                  | `backend/store/instance.go`         | Thêm `CountActiveInstances(ctx)` query         |

### 3.2 Frontend Changes

| Component           | File                                    | Thay đổi                                    |
|---------------------|-----------------------------------------|----------------------------------------------|
| Instance List       | `frontend/src/views/InstanceList.vue`   | Hiển thị quota badge                         |
| Settings Page       | `frontend/src/views/Settings.vue`       | Hiển thị plan limits overview                |

### 3.3 Database Changes
Không cần schema migration — sử dụng `COUNT(*)` trên bảng `instance` hiện có.

---

## 4. Phụ thuộc

| Dependency          | Mô tả                                                    |
|---------------------|-----------------------------------------------------------|
| License Service     | Phải có `LicenseService` hoạt động để xác định plan       |
| Instance Store      | Bảng `instance` phải có trường status để phân biệt ACTIVE |

---

## 5. Test Cases

| Test ID    | Mô tả                                                  | Expected Result                      |
|------------|----------------------------------------------------------|--------------------------------------|
| TC-001     | FREE plan tạo instance thứ 11                           | Error: RESOURCE_EXHAUSTED            |
| TC-002     | ENTERPRISE plan tạo instance thứ 11                     | Success                              |
| TC-003     | ENTERPRISE plan tạo instance thứ 100                    | Success                              |
| TC-004     | Upgrade từ FREE → ENTERPRISE, tạo instance thứ 11       | Success (limit lifted)               |
| TC-005     | Downgrade từ ENTERPRISE → FREE khi đã có > 10 instances | Giữ nguyên instances, block tạo thêm |
| TC-006     | API CreateInstance khi vượt limit                        | HTTP 429, error message rõ ràng      |

---

## 6. Rollout Plan

| Phase   | Mô tả                           | Timeline       |
|---------|----------------------------------|----------------|
| Phase 1 | Backend feature gate + API check | Sprint 1       |
| Phase 2 | Frontend quota display           | Sprint 1       |
| Phase 3 | Terraform Provider update        | Sprint 2       |
| Phase 4 | E2E testing + documentation      | Sprint 2       |
