# Change Request: Interface-Based Dependency Injection & Mock Infrastructure

| Field              | Value                                                              |
|--------------------|--------------------------------------------------------------------|
| **CR ID**          | CR-AI-002                                                          |
| **Issue IDs**      | AI-BLOCKER-002, AI-BLOCKER-006                                     |
| **Title**          | Migrate Services to Interface-Based DI & Generate Mock Test Infra  |
| **Category**       | Architecture / Testability (ARCH)                                  |
| **Priority**       | P0 — Critical                                                      |
| **Status**         | Draft                                                              |
| **Created**        | 2026-05-09                                                         |
| **Author**         | VNP AI Ops Team                                                    |
| **PRD Refs**       | ADM-08 (API Integration), SEC-01 (IAM), DCM-01 (Issue/Plan/Rollout) |

---

## 1. Tổng quan

### 1.1 Mô tả
Chuyển đổi 76 files từ inject concrete `*store.Store` sang granular interfaces (`UserReader`, `ProjectReader`, etc.) đã được định nghĩa sẵn trong `store/interfaces.go`. Đồng thời generate mock implementations và scaffold unit test suite cho 5 critical services.

### 1.2 Bối cảnh
Bytebase Store Layer (PRD §2 — Core Backend) đã định nghĩa interface-driven design trong `store/interfaces.go` với 18 granular interfaces:

```
UserReader, UserWriter, UserStore, ProjectReader, ProjectWriter, PlanReader,
IssueReader, DatabaseReader, InstanceReader, PolicyReader, SettingReader,
WorkspaceReader, AuditLogWriter, DBSchemaReader, SheetReader, RoleReader,
ChangelogReader, DataStore
```

Tuy nhiên, toàn bộ API service layer (30+ services trong PRD §2) bypass các interfaces này, trực tiếp inject `*store.Store` — khiến:
- AI agent phải resolve toàn bộ 18K LOC store layer cho mỗi service
- Mock file `store/mock/mock_store.go` chưa được generate (0 bytes)
- Zero unit tests cho critical services (auth, sql, rollout, database)

### 1.3 Mục tiêu
- 0 files import concrete `*store.Store` trong service layer
- Mock file generated và functional
- ≥60% test coverage cho top 5 critical services
- AI agent có thể viết tests mà không cần hiểu toàn bộ store

---

## 2. Yêu cầu chức năng

### FR-001: Generate Mock Implementations
- **Mô tả**: Chạy `go generate` để tạo `mock_store.go` từ interfaces đã định nghĩa
- **Logic**:
  ```bash
  go install go.uber.org/mock/mockgen@latest
  go generate ./backend/store/mock/...
  ```
- **PRD Alignment**: Hỗ trợ quality assurance cho tất cả features trong PRD §3
- **Acceptance Criteria**:
  - AC-1: File `backend/store/mock/mock_store.go` được generate thành công
  - AC-2: Mock covers tất cả 18 interfaces trong `store/interfaces.go`
  - AC-3: `go build ./backend/store/mock/...` compile clean

### FR-002: Migrate Auth Service to Interface DI
- **Mô tả**: Refactor `AuthService` constructor từ concrete sang interfaces
- **Logic**:
  ```go
  // Before
  type AuthService struct {
      store *store.Store
      // ...
  }
  func NewAuthService(store *store.Store, ...) *AuthService

  // After
  type AuthService struct {
      users    store.UserStore
      settings store.SettingReader
      workspace store.WorkspaceReader
      // ...
  }
  func NewAuthService(users store.UserStore, settings store.SettingReader, ...) *AuthService
  ```
- **PRD Alignment**: SEC-01 (IAM), SEC-05 (SSO), SEC-11 (Enterprise SSO), SEC-12 (2FA)
- **Acceptance Criteria**:
  - AC-1: `AuthService` không import `*store.Store`
  - AC-2: Constructor accepts only necessary interfaces
  - AC-3: Server wiring (`server.go`) provides concrete `*store.Store` as interface implementations

### FR-003: Migrate SQL Service to Interface DI
- **Mô tả**: Tương tự FR-002 cho `SQLService`
- **PRD Alignment**: SQL-01 (SQL Editor), SQL-02 (Admin Mode), SEC-15 (Data Masking)
- **Acceptance Criteria**: Same pattern as FR-002

### FR-004: Migrate Rollout/Database Service to Interface DI
- **Mô tả**: Tương tự cho `RolloutService` và `DatabaseService`
- **PRD Alignment**: DCM-01 (Issue/Plan/Rollout), DCM-06 (SQL Review)

### FR-005: Scaffold Unit Test Suite
- **Mô tả**: Tạo test files cho 5 critical services sử dụng generated mocks
- **Logic**: Template pattern:
  ```go
  func TestAuthService_Login(t *testing.T) {
      ctrl := gomock.NewController(t)
      defer ctrl.Finish()

      mockUsers := mock.NewMockUserStore(ctrl)
      mockUsers.EXPECT().GetUser(gomock.Any(), gomock.Any()).
          Return(&store.UserMessage{Email: "test@example.com"}, nil)

      svc := NewAuthService(mockUsers, ...)
      resp, err := svc.Login(ctx, req)
      require.NoError(t, err)
      assert.Equal(t, "test@example.com", resp.Msg.GetUser().GetEmail())
  }
  ```
- **Test Files**:
  | File | Coverage Target | Key Test Cases |
  |------|----------------|----------------|
  | `auth_service_test.go` | ≥60% | Login, MFA verify, session refresh, rate limiting |
  | `sql_service_test.go` | ≥50% | Query execution, masking apply, access check |
  | `rollout_service_test.go` | ≥50% | State transitions, task scheduling |
  | `database_service_test.go` | ≥50% | Schema sync, metadata retrieval |
  | `plan_service_test.go` | ≥50% | Plan creation, spec validation |

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                      | Thay đổi                                           |
|------------------------|-------------------------------------------|------------------------------------------------------|
| Mock Generation        | `backend/store/mock/generate.go`          | Verify `//go:generate` directive is correct          |
| Mock Output            | `backend/store/mock/mock_store.go`        | Generated — DO NOT manually edit                     |
| Auth Service           | `backend/api/v1/auth_service.go`          | Replace `*store.Store` with interfaces               |
| SQL Service            | `backend/api/v1/sql_service.go`           | Replace `*store.Store` with interfaces               |
| Rollout Service        | `backend/api/v1/rollout_service.go`       | Replace `*store.Store` with interfaces               |
| Database Service       | `backend/api/v1/database_service.go`      | Replace `*store.Store` with interfaces               |
| ACL Interceptor        | `backend/api/v1/acl.go`                   | Replace `*store.Store` with `store.DataStore`         |
| Server Wiring          | `backend/server/grpc_routes.go`           | Pass `*store.Store` as interface at wiring point     |
| Auth Tests             | `backend/api/v1/auth_service_test.go`     | NEW — mock-based unit tests                          |
| SQL Tests              | `backend/api/v1/sql_service_test.go`      | NEW — mock-based unit tests                          |
| Rollout Tests          | `backend/api/v1/rollout_service_test.go`  | NEW — mock-based unit tests                          |
| Database Tests         | `backend/api/v1/database_service_test.go` | NEW — mock-based unit tests                          |
| Plan Tests             | `backend/api/v1/plan_service_test.go`     | NEW — mock-based unit tests                          |

### 3.2 Interface Mapping per Service

| Service          | Required Interfaces                                              |
|------------------|------------------------------------------------------------------|
| AuthService      | `UserStore`, `SettingReader`, `WorkspaceReader`                  |
| SQLService       | `DatabaseReader`, `InstanceReader`, `PolicyReader`, `SheetReader` |
| RolloutService   | `PlanReader`, `IssueReader`, `DatabaseReader`                    |
| DatabaseService  | `DatabaseReader`, `InstanceReader`, `DBSchemaReader`, `ChangelogReader` |
| PlanService      | `PlanReader`, `ProjectReader`, `SheetReader`                     |

### 3.3 Không có Database Changes

---

## 4. Phụ thuộc

| Dependency             | Mô tả                                                        |
|------------------------|---------------------------------------------------------------|
| `go.uber.org/mock`     | Mock generation framework (already in `generate.go`)          |
| `store/interfaces.go`  | Interfaces already defined — no new interface creation needed |
| CR-AI-001              | File decomposition makes this easier but is NOT a blocker     |

---

## 5. Test Cases

| Test ID | Mô tả                                                            | Expected Result                     |
|---------|-------------------------------------------------------------------|-------------------------------------|
| TC-001  | `go generate ./backend/store/mock/...` succeeds                  | `mock_store.go` generated           |
| TC-002  | `go build ./backend/...` compiles after interface migration      | Zero errors                         |
| TC-003  | `go test ./backend/api/v1/...` — all new + existing tests pass  | 100% pass rate                      |
| TC-004  | Auth: TestLogin with valid credentials                           | Success response                    |
| TC-005  | Auth: TestLogin with invalid credentials                         | Error response, rate limit applied  |
| TC-006  | SQL: TestQuery with masking policy                               | Masked result returned              |
| TC-007  | Rollout: TestCreateRollout state transition                      | Correct state machine flow          |
| TC-008  | Database: TestSyncDatabase metadata update                       | Schema metadata refreshed           |
| TC-009  | Server wiring: verify `*store.Store` satisfies all interfaces    | Compile-time check passes           |
| TC-010  | `go vet ./backend/...` — no interface satisfaction errors        | Clean                               |

---

## 6. Rollout Plan

| Phase   | Mô tả                                                  | Timeline  |
|---------|----------------------------------------------------------|-----------|
| Phase 1 | Generate `mock_store.go` — verify compilation           | Sprint 1  |
| Phase 2 | Migrate `auth_service.go` + write tests                 | Sprint 1  |
| Phase 3 | Migrate `sql_service.go` + `acl.go` + write tests       | Sprint 2  |
| Phase 4 | Migrate `rollout_service.go` + `database_service.go`    | Sprint 2  |
| Phase 5 | Migrate remaining 71 files incrementally                | Sprint 3-4 |
| Phase 6 | Coverage report + CI gate enforcement (≥60%)            | Sprint 4  |

---

## 7. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                           |
|-----------------------------------------------|--------|-------------------------------------------------------|
| Some service methods use multiple store areas  | HIGH   | Introduce composite interface `DataStore` for broad access |
| Server wiring complexity increases            | MEDIUM | Single wiring point in `grpc_routes.go` — document clearly |
| Generated mock file is large (~5K LOC)        | LOW    | Normal for mock generation, excluded from AI analysis |
| Interface changes require mock regeneration   | LOW    | Add `go generate` to CI/CD pipeline                   |

---

## 8. Success Metrics

| Metric                                | Before | Target  |
|---------------------------------------|--------|---------|
| Files importing concrete `*store.Store` | 76    | ≤5 (server wiring only) |
| Mock file generated                   | No     | Yes     |
| Service unit test files               | 3      | 8+      |
| Critical service test coverage        | 0%     | ≥60%    |
| AI test generation accuracy           | ~20%   | ≥80%    |
