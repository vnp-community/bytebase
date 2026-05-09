# Solution: Interface-Based DI & Mock Test Infrastructure — CR-AI-002

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-AI-002                                               |
| **CR Reference**   | CR-AI-002                                                |
| **Title**          | Granular Interface Injection & Mock-Based Test Scaffold   |
| **Affected Layers**| L4 (Service), L8 (Store), L2 (Gateway wiring)            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §9 (L8): Store has 74 files providing typed methods per entity. `store/interfaces.go` already defines 18 granular interfaces. The concrete `*store.Store` struct implements ALL of them.

Per TDD.md §4.1: Store struct holds `dbConnManager`, `enableCache`, and 11 LRU caches. Injecting the full struct forces services to depend on cache/connection logic they don't need.

Per architecture.md §13 (Dependency Matrix): L8 (Store) is the most depended-upon layer — ALL upper layers (L3-L7) depend on it. This makes interface isolation critical for testability.

Per TDD.md §2: Server bootstrap (`grpc_routes.go` step 11) wires all 30+ ConnectRPC services — this is the single injection point.

---

## 2. Solution Design

### 2.1 Phase 1 — Generate Mocks from Existing Interfaces

**Prerequisite**: `store/interfaces.go` already exists with 18 interfaces. The mock generation infrastructure (`store/mock/generate.go`) references `go.uber.org/mock`.

```bash
# Step 1: Install mockgen
go install go.uber.org/mock/mockgen@latest

# Step 2: Verify generate.go directive
cat backend/store/mock/generate.go
# Expected: //go:generate mockgen -source=../interfaces.go -destination=mock_store.go -package=mock

# Step 3: Generate
cd backend && go generate ./store/mock/...

# Step 4: Verify
go build ./store/mock/...
```

**Output**: `backend/store/mock/mock_store.go` — ~5K LOC generated file containing:
```go
// MockUserStore is a mock of UserStore interface
type MockUserStore struct { ctrl *gomock.Controller; recorder *MockUserStoreMockRecorder }
func NewMockUserStore(ctrl *gomock.Controller) *MockUserStore { ... }
// ... for all 18 interfaces
```

### 2.2 Phase 2 — Facade Interface Strategy

**Problem**: Services like `AuthService` call 10+ different store methods across User, Setting, Workspace, Policy domains. Injecting 10 separate interfaces is impractical.

**Solution**: Use the existing `DataStore` aggregate interface for services with broad access, and granular interfaces for focused services.

```go
// store/interfaces.go — already defined
type DataStore interface {
    UserStore
    ProjectReader
    ProjectWriter
    DatabaseReader
    InstanceReader
    PolicyReader
    SettingReader
    WorkspaceReader
    PlanReader
    IssueReader
    DBSchemaReader
    SheetReader
    RoleReader
    ChangelogReader
    AuditLogWriter
}
```

**Service classification**:

| Service | Interface Strategy | Rationale |
|---------|-------------------|-----------|
| AuthService | `UserStore` + `SettingReader` + `WorkspaceReader` | Focused: only user CRUD, settings read, workspace config |
| SQLService | `DataStore` (aggregate) | Broad: reads databases, instances, policies, sheets |
| RolloutService | `DataStore` (aggregate) | Broad: reads plans, issues, databases, writes tasks |
| DatabaseService | `DatabaseReader` + `InstanceReader` + `DBSchemaReader` | Focused: database domain only |
| PlanService | `PlanReader` + `ProjectReader` + `SheetReader` | Focused: plan domain only |
| ACLInterceptor | `DataStore` (aggregate) | Security: needs broad read access for permission checks |

### 2.3 Phase 3 — Service Constructor Migration Pattern

**Before** (all services today):
```go
type AuthService struct {
    store          *store.Store          // ← concrete 18K LOC dependency
    licenseService *enterprise.LicenseService
    iamManager     *iam.Manager
    secret         string
    profile        *config.Profile
}

func NewAuthService(
    store *store.Store,               // ← concrete
    licenseService *enterprise.LicenseService,
    // ...
) *AuthService {
    return &AuthService{store: store, /*...*/}
}
```

**After** (interface-based):
```go
type AuthService struct {
    users     store.UserStore           // ← interface: GetUser, ListUsers, CreateUser, UpdateUser
    settings  store.SettingReader       // ← interface: GetSetting, ListSettings
    workspace store.WorkspaceReader     // ← interface: GetWorkspace
    licenseService *enterprise.LicenseService
    iamManager     *iam.Manager
    secret         string
    profile        *config.Profile
}

func NewAuthService(
    users store.UserStore,              // ← interface
    settings store.SettingReader,       // ← interface
    workspace store.WorkspaceReader,    // ← interface
    licenseService *enterprise.LicenseService,
    // ...
) *AuthService {
    return &AuthService{users: users, settings: settings, workspace: workspace, /*...*/}
}
```

**Method migration** (within auth_service.go):
```go
// Before
func (s *AuthService) Login(...) {
    user, err := s.store.GetUser(ctx, find)  // s.store is *store.Store
}

// After
func (s *AuthService) Login(...) {
    user, err := s.users.GetUser(ctx, find)  // s.users is store.UserStore interface
}
```

### 2.4 Phase 4 — Server Wiring Point

Per TDD.md §2 (Bootstrap Sequence, step 11): `configureGrpcRouters()` wires all services.

**Per architecture.md §3 (L2)**: `grpc_routes.go` (16.6KB) registers 30+ ConnectRPC services.

```go
// backend/server/grpc_routes.go — single wiring point
func (s *Server) configureGrpcRouters() {
    // Before: pass s.store directly
    authService := v1.NewAuthService(s.store, s.licenseService, ...)

    // After: pass concrete *store.Store which satisfies all interfaces
    authService := v1.NewAuthService(
        s.store,  // satisfies store.UserStore
        s.store,  // satisfies store.SettingReader
        s.store,  // satisfies store.WorkspaceReader
        s.licenseService,
        // ...
    )
}
```

**Compile-time verification** (add to `grpc_routes.go`):
```go
// Verify *store.Store satisfies all required interfaces at compile time
var _ store.UserStore = (*store.Store)(nil)
var _ store.SettingReader = (*store.Store)(nil)
var _ store.WorkspaceReader = (*store.Store)(nil)
var _ store.DataStore = (*store.Store)(nil)
```

### 2.5 Phase 5 — Unit Test Scaffold

**Test template** (per TDD.md §7.2 — permission check flow):

```go
// backend/api/v1/auth_service_test.go
package v1_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"
    mock "github.com/bytebase/bytebase/backend/store/mock"
    store "github.com/bytebase/bytebase/backend/store"
)

func TestAuthService_Login_ValidCredentials(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockUsers := mock.NewMockUserStore(ctrl)
    mockSettings := mock.NewMockSettingReader(ctrl)
    mockWorkspace := mock.NewMockWorkspaceReader(ctrl)

    // Setup expectations
    mockUsers.EXPECT().GetUser(gomock.Any(), gomock.Any()).Return(
        &store.UserMessage{
            Email: "test@example.com",
            PasswordHash: "$2a$10$...", // bcrypt hash of "password123"
            MFAEnabled: false,
        }, nil,
    )
    mockSettings.EXPECT().GetSetting(gomock.Any(), gomock.Any()).Return(
        &store.SettingMessage{Value: "{}"}, nil,
    )

    svc := NewAuthService(mockUsers, mockSettings, mockWorkspace, nil, nil, "secret", nil)

    // Test login
    resp, err := svc.handleLogin(context.Background(), &v1pb.LoginRequest{
        Email:    "test@example.com",
        Password: "password123",
    })
    require.NoError(t, err)
    assert.Equal(t, "test@example.com", resp.Msg.GetUser().GetEmail())
}

func TestAuthService_Login_InvalidCredentials(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockUsers := mock.NewMockUserStore(ctrl)
    mockUsers.EXPECT().GetUser(gomock.Any(), gomock.Any()).Return(nil, nil) // user not found

    svc := NewAuthService(mockUsers, nil, nil, nil, nil, "secret", nil)

    _, err := svc.handleLogin(context.Background(), &v1pb.LoginRequest{
        Email:    "notfound@example.com",
        Password: "wrong",
    })
    require.Error(t, err)
}

func TestAuthService_Login_MFARequired(t *testing.T) {
    // Per PRD SEC-12: 2FA enforcement
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockUsers := mock.NewMockUserStore(ctrl)
    mockUsers.EXPECT().GetUser(gomock.Any(), gomock.Any()).Return(
        &store.UserMessage{
            Email: "mfa@example.com",
            MFAEnabled: true,
        }, nil,
    )

    svc := NewAuthService(mockUsers, nil, nil, nil, nil, "secret", nil)

    resp, err := svc.handleLogin(context.Background(), &v1pb.LoginRequest{
        Email:    "mfa@example.com",
        Password: "correct-password",
    })
    require.NoError(t, err)
    assert.True(t, resp.Msg.GetMfaRequired())
}
```

### 2.6 CI Integration — go generate in Pipeline

```yaml
# .github/workflows/test.yml
- name: Generate mocks
  run: |
    go install go.uber.org/mock/mockgen@latest
    go generate ./backend/store/mock/...

- name: Verify mocks up-to-date
  run: |
    git diff --exit-code backend/store/mock/mock_store.go || \
      (echo "ERROR: mock_store.go is out of date. Run 'go generate ./backend/store/mock/...'" && exit 1)
```

---

## 3. Incremental Migration Order

| Step | Service | Interface(s) | Risk | Test Priority |
|------|---------|-------------|------|---------------|
| 1 | Generate mocks | N/A | None | Build verification |
| 2 | `AuthService` | `UserStore`, `SettingReader`, `WorkspaceReader` | Medium | Login, MFA, SSO |
| 3 | `DatabaseService` | `DatabaseReader`, `InstanceReader`, `DBSchemaReader` | Medium | Schema sync |
| 4 | `PlanService` | `PlanReader`, `ProjectReader`, `SheetReader` | Low | Plan CRUD |
| 5 | `SQLService` | `DataStore` (aggregate) | High | Query, masking |
| 6 | `RolloutService` | `DataStore` (aggregate) | High | State machine |
| 7 | `ACLInterceptor` | `DataStore` (aggregate) | Critical | Permission checks |
| 8 | Remaining 69 files | Mixed | Low | Batch migration |

---

## 4. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/store/mock/mock_store.go` | GENERATE | ~5K LOC mock implementations |
| `backend/api/v1/auth_service.go` | MODIFY | Replace `*store.Store` → interfaces |
| `backend/api/v1/sql_service.go` | MODIFY | Replace `*store.Store` → `DataStore` |
| `backend/api/v1/rollout_service.go` | MODIFY | Replace `*store.Store` → `DataStore` |
| `backend/api/v1/database_service.go` | MODIFY | Replace `*store.Store` → interfaces |
| `backend/api/v1/acl.go` | MODIFY | Replace `*store.Store` → `DataStore` |
| `backend/server/grpc_routes.go` | MODIFY | Update wiring to pass interfaces |
| `backend/api/v1/auth_service_test.go` | NEW | Mock-based unit tests |
| `backend/api/v1/sql_service_test.go` | NEW | Mock-based unit tests |
| `backend/api/v1/rollout_service_test.go` | NEW | Mock-based unit tests |
| `backend/api/v1/database_service_test.go` | NEW | Mock-based unit tests |
| `backend/api/v1/plan_service_test.go` | NEW | Mock-based unit tests |

---

## 5. Layer Compliance Check

Per architecture.md §13 (Dependency Matrix):
- L4 → L8: ✅ Services still depend on Store — through interfaces instead of concrete
- L2 → L4: ✅ Gateway wiring passes concrete `*store.Store` satisfying interfaces
- L3 → L5 → L8: ✅ ACL interceptor uses `DataStore` aggregate interface

**Dependency direction preserved** — interfaces defined in L8, consumed by L4.

---

## 6. Rollback Strategy

- Mock generation: Delete `mock_store.go` — no impact
- Interface migration: Revert constructor signatures + field access — `git revert`
- Test files: Delete new test files — no impact on production code
