# Change Request: Store Interface Extraction

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-001                                              |
| **Source IDs**     | ARCH-LIM-001, ARCH-WEAK-001                              |
| **Title**          | Store Interface Extraction — Break God Object Coupling   |
| **Category**       | Architecture (Testability + Modularity)                  |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | SEC-01 (IAM), DCM-01 (Change Workflow), ADM-08 (API)    |

---

## 1. Tổng quan

### 1.1 Mô tả
Tách `*store.Store` (God Object, 200+ methods, 73 file phụ thuộc) thành **role-based interfaces** theo Dependency Inversion Principle. Services sẽ depend on interface contracts thay vì concrete Store struct.

### 1.2 Bối cảnh
- `*store.Store` là central coupling point cho toàn bộ hệ thống (L3-L10)
- **0 interface definitions** trong `backend/store/` — không thể mock
- 905 source files nhưng test coverage chỉ tập trung ở plugin layer (L7)
- Service layer (L4, 36,812 lines) gần như không có unit test

### 1.3 Mục tiêu
- Giảm Store coupling từ 73 files → < 20 files dùng concrete type
- Enable mock-based unit testing cho 30+ services
- Establish role-based interfaces (Reader/Writer per entity)
- Zero breaking changes — concrete Store vẫn implement tất cả interfaces

---

## 2. Yêu cầu chức năng

### FR-001: Role-Based Interface Definitions
- **Mô tả**: Tạo file `backend/store/interfaces.go` chứa domain-specific interfaces.
- **Logic**:
  ```go
  // interfaces.go — role-based contracts
  type UserReader interface {
      GetUser(ctx, *FindUserMessage) (*UserMessage, error)
      ListUsers(ctx, *FindUserMessage) ([]*UserMessage, error)
  }
  type UserWriter interface {
      CreateUser(ctx, *UserMessage) (*UserMessage, error)
      UpdateUser(ctx, uid int, *UpdateUserMessage) (*UserMessage, error)
  }
  type PlanReader interface {
      GetPlan(ctx, *FindPlanMessage) (*PlanMessage, error)
      ListPlans(ctx, *FindPlanMessage) ([]*PlanMessage, error)
  }
  // ... 12+ interfaces per entity domain
  ```
- **Acceptance Criteria**:
  - AC-1: Mỗi entity domain có tối thiểu Reader + Writer interface
  - AC-2: Interface methods map 1:1 với existing Store public methods
  - AC-3: `*Store` struct satisfy tất cả interfaces (compile-time verified)
  - AC-4: No breaking changes — existing code vẫn compile

### FR-002: Service Dependency Injection Migration
- **Mô tả**: Migrate 30+ services từ `*store.Store` sang interface dependencies.
- **Logic**:
  ```go
  // BEFORE:
  type AuthService struct { store *store.Store }

  // AFTER:
  type AuthService struct {
      userReader  store.UserReader
      userWriter  store.UserWriter
      tokenStore  store.TokenStore
  }
  ```
- **Acceptance Criteria**:
  - AC-1: AuthService chỉ depend on 3-4 interfaces (thay vì 200+ methods)
  - AC-2: Service constructors accept interfaces, wired in grpc_routes.go
  - AC-3: Minimum 5 services migrated in Phase 1

### FR-003: Component Layer Interface Extraction
- **Mô tả**: Extract interfaces cho `iam.Manager` và `enterprise.LicenseService`.
- **Logic**:
  ```go
  // iam/interfaces.go
  type PermissionChecker interface {
      CheckPermission(ctx, permission, user, workspaceID string, projectIDs ...string) (bool, error)
  }
  // enterprise/interfaces.go
  type FeatureChecker interface {
      IsFeatureEnabled(ctx, workspaceID string, feature v1pb.PlanFeature) error
  }
  ```
- **Acceptance Criteria**:
  - AC-1: Services depend on `iam.PermissionChecker` thay vì `*iam.Manager`
  - AC-2: `*iam.Manager` implement `PermissionChecker` interface
  - AC-3: Mock generation via `go generate` + `mockgen`

### FR-004: Mock Infrastructure
- **Mô tả**: Setup `mockgen` + generated mocks cho unit testing.
- **Logic**:
  ```go
  //go:generate mockgen -source=interfaces.go -destination=mock/mock_store.go -package=mock
  ```
- **Acceptance Criteria**:
  - AC-1: Generated mock files checked in tại `backend/store/mock/`
  - AC-2: Ví dụ unit test cho AuthService sử dụng mock
  - AC-3: CI chạy `go generate` và verify mocks up-to-date

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Interface definitions  | `backend/store/interfaces.go`         | 12+ domain interfaces (Reader/Writer)        |
| IAM interfaces         | `backend/component/iam/interfaces.go` | PermissionChecker, RoleResolver              |
| Enterprise interfaces  | `backend/enterprise/interfaces.go`    | FeatureChecker, PlanChecker                  |
| Mock generation        | `backend/store/mock/`                 | mockgen-generated mocks                      |
| Service migration      | `backend/api/v1/*_service.go`         | Constructor params: concrete → interface     |
| Wiring                 | `backend/server/grpc_routes.go`       | Pass concrete Store as interface             |

### 3.2 Database Changes
Không có.

### 3.3 Frontend Changes
Không có.

---

## 4. Phụ thuộc

| Dependency              | Mô tả                                              |
|-------------------------|-----------------------------------------------------|
| `go.uber.org/mock`      | Mock generation tool (mockgen)                      |
| Go 1.26                 | Interface embedding support                         |

---

## 5. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | `*Store` satisfies all interfaces (compile-time)             | Build passes                             |
| TC-002     | AuthService unit test with mock UserReader                   | Test passes without DB                   |
| TC-003     | PlanService unit test with mock PlanReader                   | Test passes without DB                   |
| TC-004     | grpc_routes.go wires concrete Store as interface deps        | Server starts normally                   |
| TC-005     | Integration tests still pass (backward compatible)           | Zero regression                          |
| TC-006     | `go generate` produces up-to-date mocks                     | CI check passes                          |
| TC-007     | Service using old `*store.Store` still compiles              | Gradual migration safe                   |

---

## 6. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Create `interfaces.go` + compile-time verification | Sprint 1     |
| Phase 2 | Migrate AuthService + mock tests (POC)             | Sprint 1     |
| Phase 3 | Migrate PlanService, IssueService, RolloutService  | Sprint 2     |
| Phase 4 | IAM + Enterprise interface extraction              | Sprint 2     |
| Phase 5 | Migrate remaining 25+ services                     | Sprint 3-4   |
| Phase 6 | CI coverage gate (60% unit test target)            | Sprint 4     |

---

## 7. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Interface churn during active development     | MEDIUM | Start with stable entity interfaces (User, Plan)    |
| Constructor signature changes break callers   | HIGH   | Phase migration, old constructors deprecated first  |
| Mock divergence from real Store               | LOW    | `go generate` CI check ensures sync                  |
| Large PR size for interface file              | MEDIUM | One interface domain per PR                          |
