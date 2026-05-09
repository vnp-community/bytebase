# Solution: Test Coverage Hardening — CR-WEAK-007

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-007                                             |
| **CR Reference**   | CR-WEAK-007                                              |
| **Title**          | Test Infrastructure Rearchitecture + Coverage Enforcement |
| **Affected Layers**| L4 (Service), L5 (Component), L8 (Store)                 |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §5 (L4): 79 service files, ~1MB+ code. Services depend on L5 (components), L7 (plugins), L8 (store), L9 (enterprise).

Per TDD.md §2: Server bootstrap wires everything together in `NewServer()`. This tight coupling makes unit testing difficult — services receive concrete types, not interfaces.

**Current test state** (verified):
- `changelog_test.go` — **14 bytes** (empty!)
- `auth_service.go` (1930 lines) — **no unit test file**
- `sql_service.go` (1876 lines) — **no unit test file**
- Integration tests (`backend/tests/`) — 42 files, all require Docker testcontainers

---

## 2. Architectural Problem: Missing Interface Layer

The root cause of low test coverage is **architectural**: services depend on concrete types, not interfaces. This makes mocking impossible without an interface extraction.

```
CURRENT (untestable):
  AuthService → *store.Store (concrete)
             → *enterprise.LicenseService (concrete)
             → *iam.Manager (concrete)

PROPOSED (testable):
  AuthService → store.Reader + store.Writer (interfaces)
             → enterprise.FeatureChecker (interface)
             → iam.PermissionChecker (interface)
```

---

## 3. Solution Design

### 3.1 Architecture Change: Store Interface Extraction

**New file**: `backend/store/interfaces.go`

```go
package store

import "context"

// UserReader provides read access to user data.
// Extracted from Store to enable unit testing without database.
type UserReader interface {
    GetUser(ctx context.Context, find *FindUserMessage) (*UserMessage, error)
    GetUserByEmail(ctx context.Context, workspace, email string) (*UserMessage, error)
    ListUsers(ctx context.Context, find *FindUserMessage) ([]*UserMessage, error)
}

// UserWriter provides write access to user data.
type UserWriter interface {
    CreateUser(ctx context.Context, create *UserMessage) (*UserMessage, error)
    UpdateUser(ctx context.Context, id int, patch *UpdateUserMessage) (*UserMessage, error)
}

// PlanReader provides read access to plan data.
type PlanReader interface {
    GetPlan(ctx context.Context, find *FindPlanMessage) (*PlanMessage, error)
    ListPlans(ctx context.Context, find *FindPlanMessage) ([]*PlanMessage, error)
}

// IssueReader provides read access to issue data.
type IssueReader interface {
    GetIssue(ctx context.Context, find *FindIssueMessage) (*IssueMessage, error)
    ListIssues(ctx context.Context, find *FindIssueMessage) ([]*IssueMessage, error)
}

// ChangelogWriter provides write access to changelog data.
type ChangelogWriter interface {
    CreateChangelog(ctx context.Context, create *ChangelogMessage) (int64, error)
    UpdateChangelog(ctx context.Context, update *UpdateChangelogMessage) error
}

// PolicyReader provides read access to policy data.
type PolicyReader interface {
    GetPolicy(ctx context.Context, find *FindPolicyMessage) (*PolicyMessage, error)
    ListPolicies(ctx context.Context, find *FindPolicyMessage) ([]*PolicyMessage, error)
}

// Verify Store implements all interfaces at compile time.
var (
    _ UserReader      = (*Store)(nil)
    _ UserWriter      = (*Store)(nil)
    _ PlanReader      = (*Store)(nil)
    _ IssueReader     = (*Store)(nil)
    _ ChangelogWriter = (*Store)(nil)
    _ PolicyReader    = (*Store)(nil)
)
```

**Design rationale**: Instead of one massive `StoreInterface`, we use **role-based interfaces** (Reader/Writer per entity). Services declare only the interfaces they need → minimal mock surface.

### 3.2 Architecture Change: Component Interfaces

**New file**: `backend/component/iam/interfaces.go`

```go
package iam

import (
    "context"
    "github.com/bytebase/bytebase/backend/common/permission"
    "github.com/bytebase/bytebase/backend/store"
)

// PermissionChecker is the interface for IAM permission checking.
// Extracted to enable unit testing of ACL interceptor and services.
type PermissionChecker interface {
    CheckPermission(ctx context.Context, p permission.Permission, user *store.UserMessage, workspaceID string, projectIDs ...string) (bool, error)
}

// Verify Manager implements PermissionChecker.
var _ PermissionChecker = (*Manager)(nil)
```

**New file**: `backend/enterprise/interfaces.go`

```go
package enterprise

import (
    "context"
    v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
)

// FeatureChecker checks if an enterprise feature is enabled.
type FeatureChecker interface {
    IsFeatureEnabled(ctx context.Context, workspaceID string, feature v1pb.PlanFeature) error
}

var _ FeatureChecker = (*LicenseService)(nil)
```

### 3.3 Mock Generation

**New directory**: `backend/testutil/mocks/`

Using `go generate` with `mockgen`:

```go
// backend/store/interfaces.go
//go:generate mockgen -source=interfaces.go -destination=../testutil/mocks/mock_store.go -package=mocks

// backend/component/iam/interfaces.go
//go:generate mockgen -source=interfaces.go -destination=../../testutil/mocks/mock_iam.go -package=mocks

// backend/enterprise/interfaces.go
//go:generate mockgen -source=interfaces.go -destination=../testutil/mocks/mock_enterprise.go -package=mocks
```

### 3.4 Auth Service Unit Tests — Example

**New file**: `backend/api/v1/auth_service_test.go`

```go
package v1

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"

    "github.com/bytebase/bytebase/backend/testutil/mocks"
    "github.com/bytebase/bytebase/backend/store"
)

func TestLogin_Success(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockStore := mocks.NewMockUserReader(ctrl)
    mockStore.EXPECT().
        GetUserByEmail(gomock.Any(), "ws-1", "admin@example.com").
        Return(&store.UserMessage{
            Email:        "admin@example.com",
            PasswordHash: hashPassword("correct-password"),
            Status:       store.Active,
        }, nil)

    svc := &AuthService{userReader: mockStore}
    // ...test login flow...
}

func TestLogin_WrongPassword(t *testing.T) {
    // Mock returns user → password mismatch → error
}

func TestLogin_DisabledUser(t *testing.T) {
    // Mock returns user with Status=Disabled → error
}

func TestLogin_StoreError(t *testing.T) {
    // Mock returns error → service returns 503
}
```

### 3.5 Fix Empty Test Files

**`backend/store/changelog_test.go`** — currently 14 bytes:

```go
package store

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCreateChangelog_Validation(t *testing.T) {
    // Test that CreateChangelog rejects nil fields
    msg := &ChangelogMessage{}
    assert.Error(t, validateChangelogMessage(msg))
}

func TestUpdateChangelog_StatusTransition(t *testing.T) {
    // Test valid status transitions: PENDING → DONE, PENDING → FAILED
}

func TestChangelogMessage_ProjectRequired(t *testing.T) {
    // Test that composite PK validation requires project
}
```

### 3.6 Component Layer Tests

**New file**: `backend/component/bus/bus_test.go`

```go
package bus

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestBus_TaskRunTickle_BufferOverflow(t *testing.T) {
    b, err := New()
    require.NoError(t, err)

    // Fill buffer (1000)
    for i := 0; i < 1000; i++ {
        b.TaskRunTickleChan <- i
    }

    // Next send should NOT block (use select with timeout)
    done := make(chan bool, 1)
    go func() {
        select {
        case b.TaskRunTickleChan <- 1001:
            done <- true
        case <-time.After(100 * time.Millisecond):
            done <- false // buffer full, send blocked
        }
    }()
    assert.False(t, <-done, "send should block when buffer full")
}

func TestBus_CancelFuncMap_ConcurrentAccess(t *testing.T) {
    // Test sync.Map concurrent store/load/delete
}
```

### 3.7 CI Coverage Gate

**New file**: `.github/workflows/coverage.yml`

```yaml
name: Coverage Gate
on: [pull_request]

jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Run unit tests with coverage
        run: |
          go test -coverprofile=coverage.out -covermode=atomic \
            ./backend/api/v1/... \
            ./backend/component/... \
            ./backend/store/...

      - name: Check coverage thresholds
        run: |
          go tool cover -func=coverage.out | \
          awk '/^total:/ { 
            gsub(/%/, "", $3); 
            if ($3 < 50) { 
              print "FAIL: total coverage " $3 "% < 50%"; 
              exit 1 
            } 
          }'

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: coverage.out
```

---

## 4. Architecture Impact

### Dependency Direction Change

```
BEFORE:
  L4 (Service) → L8 (Store) [concrete *store.Store]

AFTER:
  L4 (Service) → store.UserReader (interface)    ← defined in L8
                → iam.PermissionChecker (interface) ← defined in L5
                → enterprise.FeatureChecker (interface) ← defined in L9
  
  Wiring: L2 (server.go) injects concrete implementations
```

This follows the **Dependency Inversion Principle** — L4 depends on abstractions, not concretions. The wiring happens in `server.go` (L2), which already creates all components.

### Gradual Migration Strategy

Services don't need to change all at once. New interface fields can coexist:

```go
type AuthService struct {
    store      *store.Store           // legacy (for methods not yet in interfaces)
    userReader store.UserReader       // new (for testable methods)
    iamChecker iam.PermissionChecker  // new
}
```

---

## 5. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/store/interfaces.go` | L8 | NEW | Store interface extraction |
| `backend/component/iam/interfaces.go` | L5 | NEW | IAM interface |
| `backend/enterprise/interfaces.go` | L9 | NEW | Enterprise interface |
| `backend/testutil/mocks/` | — | NEW | Generated mocks |
| `backend/api/v1/auth_service_test.go` | L4 | NEW | Auth tests |
| `backend/store/changelog_test.go` | L8 | MODIFY | Replace empty file |
| `backend/component/bus/bus_test.go` | L5 | NEW | Bus tests |
| `.github/workflows/coverage.yml` | CI | NEW | Coverage gate |

## 6. Rollback

Interface files are additive — removing them doesn't break existing code. Mock files can be deleted. Coverage CI can be disabled. No database changes.
