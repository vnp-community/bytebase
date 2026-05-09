# Solution: Store Interface Extraction — CR-ARCH-001

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-001                                             |
| **CR Reference**   | CR-ARCH-001                                              |
| **Title**          | Role-Based Interface Extraction + Mock Infrastructure    |
| **Affected Layers**| L4 (Service), L5 (Component), L8 (Store)                 |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §8 (L8 — Data Access Layer):
- `*store.Store` chứa 200+ public methods, 13 LRU caches, single `DBConnectionManager`
- 73 files depend trực tiếp trên concrete `*store.Store`

Per [TDD.md](../../TDD.md) §4 (Store Layer):
- Store pattern: single struct + per-entity files (`user.go`, `plan.go`, ...)
- Cache strategy: per-entity LRU với `enableCache` toggle

---

## 2. Current Implementation Analysis

### 2.1 Store Struct (store.go:18-39)

```go
type Store struct {
    dbConnManager *DBConnectionManager
    enableCache   bool
    Secret            string
    userEmailCache    *lru.Cache[string, *UserMessage]
    instanceCache     *lru.Cache[string, *InstanceMessage]
    databaseCache     *lru.Cache[string, *DatabaseMessage]
    projectCache      *lru.Cache[string, *ProjectMessage]
    policyCache       *lru.Cache[string, *PolicyMessage]
    settingCache      *lru.Cache[string, *SettingMessage]
    rolesCache        *expirable.LRU[string, *RoleMessage]
    groupCache        *expirable.LRU[string, *GroupMessage]
    groupMembersCache *expirable.LRU[string, map[string]bool]
    memberGroupsCache *expirable.LRU[string, []string]
    dbSchemaCache     *expirable.LRU[string, *model.DatabaseMetadata]
    iamPolicyCache    *expirable.LRU[string, *IamPolicyMessage]
    sheetFullCache    *lru.Cache[string, *SheetMessage]
}
```

### 2.2 Service Coupling Example (grpc_routes.go:252)

```go
// ALL services receive the same *store.Store with 200+ methods
authService := apiv1.NewAuthService(stores, secret, licenseService, profile, iamManager)
databaseService := apiv1.NewDatabaseService(stores, schemaSyncer, profile, iamManager, licenseService)
planService := apiv1.NewPlanService(stores, licenseService, profile, iamManager)
```

### 2.3 Root Cause
Go's single-package Store design avoids import cycles but creates a God Object where every service has access to every method — violating Interface Segregation Principle.

---

## 3. Solution Design

### 3.1 Phase 1 — Role-Based Interface Definitions

**New file**: `backend/store/interfaces.go`

```go
package store

import (
    "context"
    "database/sql"

    storepb "github.com/bytebase/bytebase/backend/generated-go/store"
    "github.com/bytebase/bytebase/backend/store/model"
)

// ============================================================
// USER DOMAIN
// ============================================================

// UserReader provides read-only access to user data.
type UserReader interface {
    GetUser(ctx context.Context, find *FindUserMessage) (*UserMessage, error)
    ListUsers(ctx context.Context, find *FindUserMessage) ([]*UserMessage, error)
    CountUsers(ctx context.Context, workspaceID string) (int, error)
}

// UserWriter provides write access to user data.
type UserWriter interface {
    CreateUser(ctx context.Context, create *UserMessage) (*UserMessage, error)
    UpdateUser(ctx context.Context, workspaceID string, uid int, patch *UpdateUserMessage) (*UserMessage, error)
}

// UserStore combines reader and writer for user domain.
type UserStore interface {
    UserReader
    UserWriter
}

// ============================================================
// PROJECT DOMAIN
// ============================================================

type ProjectReader interface {
    GetProject(ctx context.Context, find *FindProjectMessage) (*ProjectMessage, error)
    ListProjects(ctx context.Context, find *FindProjectMessage) ([]*ProjectMessage, error)
}

type ProjectWriter interface {
    CreateProject(ctx context.Context, create *ProjectMessage) (*ProjectMessage, error)
    UpdateProject(ctx context.Context, workspaceID string, uid int64, patch *UpdateProjectMessage) (*ProjectMessage, error)
}

type ProjectStore interface {
    ProjectReader
    ProjectWriter
}

// ============================================================
// PLAN DOMAIN
// ============================================================

type PlanReader interface {
    GetPlan(ctx context.Context, find *FindPlanMessage) (*PlanMessage, error)
    ListPlans(ctx context.Context, find *FindPlanMessage) ([]*PlanMessage, error)
}

type PlanWriter interface {
    CreatePlan(ctx context.Context, create *PlanMessage) (*PlanMessage, error)
    UpdatePlan(ctx context.Context, workspaceID string, uid int64, patch *UpdatePlanMessage) (*PlanMessage, error)
}

type PlanStore interface {
    PlanReader
    PlanWriter
}

// ============================================================
// ISSUE DOMAIN
// ============================================================

type IssueReader interface {
    GetIssue(ctx context.Context, find *FindIssueMessage) (*IssueMessage, error)
    ListIssues(ctx context.Context, find *FindIssueMessage) (*ListIssueMessage, error)
}

type IssueWriter interface {
    CreateIssue(ctx context.Context, create *IssueMessage) (*IssueMessage, error)
    UpdateIssue(ctx context.Context, workspaceID string, uid int, patch *UpdateIssueMessage) (*IssueMessage, error)
}

type IssueStore interface {
    IssueReader
    IssueWriter
}

// ============================================================
// DATABASE DOMAIN
// ============================================================

type DatabaseReader interface {
    GetDatabase(ctx context.Context, find *FindDatabaseMessage) (*DatabaseMessage, error)
    ListDatabases(ctx context.Context, find *FindDatabaseMessage) ([]*DatabaseMessage, error)
}

type DatabaseWriter interface {
    UpdateDatabase(ctx context.Context, workspaceID string, patch *UpdateDatabaseMessage) (*DatabaseMessage, error)
}

type DatabaseStore interface {
    DatabaseReader
    DatabaseWriter
}

// ============================================================
// INSTANCE DOMAIN
// ============================================================

type InstanceReader interface {
    GetInstance(ctx context.Context, find *FindInstanceMessage) (*InstanceMessage, error)
    ListInstances(ctx context.Context, find *FindInstanceMessage) ([]*InstanceMessage, error)
}

type InstanceWriter interface {
    CreateInstance(ctx context.Context, create *InstanceMessage) (*InstanceMessage, error)
    UpdateInstance(ctx context.Context, workspaceID string, uid int, patch *UpdateInstanceMessage) (*InstanceMessage, error)
}

type InstanceStore interface {
    InstanceReader
    InstanceWriter
}

// ============================================================
// POLICY DOMAIN
// ============================================================

type PolicyReader interface {
    GetPolicy(ctx context.Context, find *FindPolicyMessage) (*PolicyMessage, error)
    ListPolicies(ctx context.Context, find *FindPolicyMessage) ([]*PolicyMessage, error)
}

type PolicyWriter interface {
    UpsertPolicy(ctx context.Context, workspaceID string, create *PolicyMessage) (*PolicyMessage, error)
    DeletePolicy(ctx context.Context, workspaceID string, find *FindPolicyMessage) error
}

type PolicyStore interface {
    PolicyReader
    PolicyWriter
}

// ============================================================
// SETTING DOMAIN
// ============================================================

type SettingReader interface {
    GetSetting(ctx context.Context, find *FindSettingMessage) (*SettingMessage, error)
    ListSettings(ctx context.Context, find *FindSettingMessage) ([]*SettingMessage, error)
    GetWorkspaceID(ctx context.Context) (string, error)
}

type SettingWriter interface {
    UpsertSetting(ctx context.Context, workspaceID string, set *SettingMessage) (*SettingMessage, error)
}

type SettingStore interface {
    SettingReader
    SettingWriter
}

// ============================================================
// IAM DOMAIN
// ============================================================

type IAMPolicyReader interface {
    GetIamPolicy(ctx context.Context, find *FindIamPolicyMessage) (*IamPolicyMessage, error)
}

type IAMPolicyWriter interface {
    SetIamPolicy(ctx context.Context, workspaceID string, set *IamPolicyMessage) (*IamPolicyMessage, error)
}

// ============================================================
// SHEET DOMAIN
// ============================================================

type SheetReader interface {
    GetSheet(ctx context.Context, find *FindSheetMessage) (*SheetMessage, error)
    ListSheets(ctx context.Context, find *FindSheetMessage) ([]*SheetMessage, error)
}

type SheetWriter interface {
    CreateSheet(ctx context.Context, create *SheetMessage) (*SheetMessage, error)
    DeleteSheet(ctx context.Context, workspaceID string, uid int) error
}

// ============================================================
// ROLLOUT DOMAIN
// ============================================================

type RolloutReader interface {
    GetRollout(ctx context.Context, uid int64) (*RolloutMessage, error)
    ListTaskRuns(ctx context.Context, find *FindTaskRunMessage) ([]*TaskRunMessage, error)
}

type RolloutWriter interface {
    CreateRollout(ctx context.Context, create *RolloutMessage) (*RolloutMessage, error)
    UpdateTaskRunStatus(ctx context.Context, patch *TaskRunStatusPatch) error
}

// ============================================================
// CHANGELOG & AUDIT DOMAIN
// ============================================================

type ChangelogReader interface {
    GetChangelog(ctx context.Context, find *FindChangelogMessage) (*ChangelogMessage, error)
    ListChangelogs(ctx context.Context, find *FindChangelogMessage) ([]*ChangelogMessage, error)
}

type AuditLogWriter interface {
    CreateAuditLog(ctx context.Context, create *AuditLogMessage) error
}

// ============================================================
// AGGREGATE — Full Store interface (for gradual migration)
// ============================================================

// FullStore is the aggregate interface matching the concrete *Store.
// Used during migration to ensure compile-time compatibility.
type FullStore interface {
    UserStore
    ProjectStore
    PlanStore
    IssueStore
    DatabaseStore
    InstanceStore
    PolicyStore
    SettingStore
    SheetReader
    SheetWriter
    RolloutReader
    RolloutWriter
    ChangelogReader
    AuditLogWriter
    // DB access for runners that need raw SQL
    GetDB() *sql.DB
    Close() error
    DeleteCache()
}
```

### 3.2 Phase 1b — Compile-Time Verification

**New file**: `backend/store/interfaces_verify_test.go`

```go
package store

// Compile-time verification that *Store implements all interfaces.
var _ UserReader = (*Store)(nil)
var _ UserWriter = (*Store)(nil)
var _ ProjectReader = (*Store)(nil)
var _ ProjectWriter = (*Store)(nil)
var _ PlanReader = (*Store)(nil)
var _ PlanWriter = (*Store)(nil)
var _ IssueReader = (*Store)(nil)
var _ IssueWriter = (*Store)(nil)
var _ DatabaseReader = (*Store)(nil)
var _ DatabaseWriter = (*Store)(nil)
var _ InstanceReader = (*Store)(nil)
var _ InstanceWriter = (*Store)(nil)
var _ PolicyReader = (*Store)(nil)
var _ PolicyWriter = (*Store)(nil)
var _ SettingReader = (*Store)(nil)
var _ SettingWriter = (*Store)(nil)
var _ FullStore = (*Store)(nil)
```

### 3.3 Phase 2 — Mock Generation Infrastructure

**New file**: `backend/store/mock/generate.go`

```go
package mock

//go:generate mockgen -source=../interfaces.go -destination=mock_store.go -package=mock
```

**Generated file**: `backend/store/mock/mock_store.go`

```go
// Code generated by MockGen. DO NOT EDIT.
// Source: interfaces.go

package mock

import (
    "context"
    reflect "reflect"

    gomock "go.uber.org/mock/gomock"
    store "github.com/bytebase/bytebase/backend/store"
)

// MockUserReader is a mock of UserReader interface.
type MockUserReader struct {
    ctrl     *gomock.Controller
    recorder *MockUserReaderMockRecorder
}

func NewMockUserReader(ctrl *gomock.Controller) *MockUserReader {
    mock := &MockUserReader{ctrl: ctrl}
    mock.recorder = &MockUserReaderMockRecorder{mock}
    return mock
}

func (m *MockUserReader) GetUser(ctx context.Context, find *store.FindUserMessage) (*store.UserMessage, error) {
    m.ctrl.T.Helper()
    ret := m.ctrl.Call(m, "GetUser", ctx, find)
    return ret[0].(*store.UserMessage), ret[1].(error)
}

// ... additional mocks generated by mockgen
```

### 3.4 Phase 3 — Service DI Migration (AuthService POC)

**Modified file**: `backend/api/v1/auth_service.go`

```go
// BEFORE — depends on God Object
type AuthService struct {
    store          *store.Store
    licenseService *enterprise.LicenseService
    profile        *config.Profile
    iamManager     *iam.Manager
    secret         string
}

func NewAuthService(
    store *store.Store,
    secret string,
    licenseService *enterprise.LicenseService,
    profile *config.Profile,
    iamManager *iam.Manager,
) *AuthService { ... }

// AFTER — depends on narrow interfaces
type AuthService struct {
    userReader     store.UserReader
    userWriter     store.UserWriter
    settingReader  store.SettingReader
    auditWriter    store.AuditLogWriter
    licenseService enterprise.FeatureChecker  // interface, not concrete
    profile        *config.Profile
    iamChecker     iam.PermissionChecker      // interface, not concrete
    secret         string
}

func NewAuthService(
    userReader store.UserReader,
    userWriter store.UserWriter,
    settingReader store.SettingReader,
    auditWriter store.AuditLogWriter,
    secret string,
    licenseService enterprise.FeatureChecker,
    profile *config.Profile,
    iamChecker iam.PermissionChecker,
) *AuthService {
    return &AuthService{
        userReader:     userReader,
        userWriter:     userWriter,
        settingReader:  settingReader,
        auditWriter:    auditWriter,
        licenseService: licenseService,
        profile:        profile,
        iamChecker:     iamChecker,
        secret:         secret,
    }
}
```

### 3.5 Phase 3b — IAM + Enterprise Interface Extraction

**New file**: `backend/component/iam/interfaces.go`

```go
package iam

import "context"

// PermissionChecker verifies user permissions.
type PermissionChecker interface {
    CheckPermission(ctx context.Context, permission Permission, user *UserInfo) error
}

// RoleResolver resolves user roles for a given context.
type RoleResolver interface {
    GetUserRoles(ctx context.Context, workspaceID, userUID string) ([]string, error)
}

// Compile-time verification
var _ PermissionChecker = (*Manager)(nil)
```

**New file**: `backend/enterprise/interfaces.go`

```go
package enterprise

import (
    "context"
    v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
)

// FeatureChecker checks if a feature is available in the current plan.
type FeatureChecker interface {
    IsFeatureEnabled(ctx context.Context, workspaceID string, feature v1pb.PlanFeature) error
}

// PlanResolver returns the current plan information.
type PlanResolver interface {
    GetCurrentPlan(ctx context.Context, workspaceID string) v1pb.PlanType
}

// Compile-time verification
var _ FeatureChecker = (*LicenseService)(nil)
```

### 3.6 Phase 4 — Wiring in grpc_routes.go

**Modified file**: `backend/server/grpc_routes.go`

```go
// BEFORE:
authService := apiv1.NewAuthService(stores, secret, licenseService, profile, iamManager)

// AFTER — concrete *Store satisfies all interfaces (no runtime change):
authService := apiv1.NewAuthService(
    stores,             // as store.UserReader (implicit interface satisfaction)
    stores,             // as store.UserWriter
    stores,             // as store.SettingReader
    stores,             // as store.AuditLogWriter
    secret,
    licenseService,     // as enterprise.FeatureChecker
    profile,
    iamManager,         // as iam.PermissionChecker
)
```

### 3.7 Phase 5 — Unit Test with Mocks

**New file**: `backend/api/v1/auth_service_test.go`

```go
package v1_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"

    v1 "github.com/bytebase/bytebase/backend/api/v1"
    mockstore "github.com/bytebase/bytebase/backend/store/mock"
    mockiam "github.com/bytebase/bytebase/backend/component/iam/mock"
    mockent "github.com/bytebase/bytebase/backend/enterprise/mock"
)

func TestAuthService_Login_Success(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    userReader := mockstore.NewMockUserReader(ctrl)
    userWriter := mockstore.NewMockUserWriter(ctrl)
    settingReader := mockstore.NewMockSettingReader(ctrl)
    auditWriter := mockstore.NewMockAuditLogWriter(ctrl)
    featureChecker := mockent.NewMockFeatureChecker(ctrl)
    permChecker := mockiam.NewMockPermissionChecker(ctrl)

    // Setup expectations
    userReader.EXPECT().
        GetUser(gomock.Any(), gomock.Any()).
        Return(&store.UserMessage{
            UID:          1,
            Email:        "test@example.com",
            PasswordHash: "$2a$10$...",  // bcrypt hash of "password123"
        }, nil)

    svc := v1.NewAuthService(
        userReader, userWriter, settingReader, auditWriter,
        "test-secret", featureChecker, testProfile, permChecker,
    )

    resp, err := svc.Login(context.Background(), /* login request */)
    require.NoError(t, err)
    require.NotEmpty(t, resp.Token)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    userReader := mockstore.NewMockUserReader(ctrl)
    userReader.EXPECT().
        GetUser(gomock.Any(), gomock.Any()).
        Return(nil, nil) // user not found

    // ... setup and verify error response
}
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/store/interfaces.go` | L8 | **NEW** | 12+ domain interfaces |
| `backend/store/interfaces_verify_test.go` | L8 | **NEW** | Compile-time verification |
| `backend/store/mock/generate.go` | L8 | **NEW** | mockgen directive |
| `backend/store/mock/mock_store.go` | L8 | **GENERATED** | Auto-generated mocks |
| `backend/component/iam/interfaces.go` | L5 | **NEW** | PermissionChecker interface |
| `backend/enterprise/interfaces.go` | L9 | **NEW** | FeatureChecker interface |
| `backend/api/v1/auth_service.go` | L4 | **MODIFY** | DI via interfaces |
| `backend/api/v1/auth_service_test.go` | L4 | **NEW** | Mock-based unit tests |
| `backend/server/grpc_routes.go` | L2 | **MODIFY** | Updated wiring |

---

## 5. Dependency Direction Validation

```
L4 (auth_service.go) → store.UserReader (L8 interface)
L4 (auth_service.go) → iam.PermissionChecker (L5 interface)
L4 (auth_service.go) → enterprise.FeatureChecker (L9 interface)
L2 (grpc_routes.go)  → *store.Store (L8 concrete — satisfies interfaces)
```

**Direction**: Upper layers depend on interfaces defined in lower layers. Concrete implementations wired at L2 (composition root). This follows Dependency Inversion Principle.

---

## 6. Migration Strategy

### Gradual Migration — No Big Bang

```
Sprint 1: Create interfaces.go + verify test → ZERO behavioral change
Sprint 1: Mock generation infra → ZERO behavioral change
Sprint 2: Migrate AuthService (POC) → 1 service changed
Sprint 2: IAM + Enterprise interfaces → 2 new interfaces
Sprint 3: Migrate PlanService, IssueService → 3 services
Sprint 4+: Remaining services → gradual
```

**Key invariant**: `*store.Store` always satisfies all interfaces. Old and new code coexist.

### Compatibility Bridge

Services not yet migrated continue using `*store.Store` directly:

```go
// Old-style service (not yet migrated):
planCheckService := apiv1.NewPlanCheckService(stores, ...)  // still uses *store.Store

// New-style service (migrated):
authService := apiv1.NewAuthService(stores, stores, stores, stores, ...)  // same concrete, narrow interfaces
```

---

## 7. Test Strategy

| Level | Test | Tool |
|-------|------|------|
| Compile-time | `var _ Interface = (*Store)(nil)` | `go build` |
| Unit (mock) | AuthService.Login with MockUserReader | `go.uber.org/mock` |
| Unit (mock) | AuthService.Login user-not-found error | `go.uber.org/mock` |
| Integration | Full flow via testcontainers | `go test -tags integration` |
| CI gate | `go generate` + `git diff --exit-code` | CI pipeline |

---

## 8. Rollback Plan

1. Interfaces are additive — removing them requires no code changes
2. If mock-based tests flaky → keep integration tests as primary
3. Service constructors can revert to `*store.Store` by removing interface params
4. No database changes → no data rollback needed
