# AI-BLOCKER-002: Services Depend on Concrete `*store.Store` Instead of Interfaces

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-002 |
| **Severity** | 🔴 Critical |
| **Category** | Dependency Coupling / Testability |
| **Layer** | L4 Service → L8 Store |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

76 files across the backend import `*store.Store` directly as a concrete dependency. While `store/interfaces.go` defines granular interfaces (`UserReader`, `ProjectReader`, `DatabaseReader`, etc.), the API service layer bypasses them and injects the full `*store.Store` struct. This means AI agents must resolve the entire 18K LOC store layer to understand any single service's data access patterns.

## Impact on AI Operations

- **Context Explosion**: To understand what `AuthService` reads/writes, AI must parse the full `store.Store` type (183 LOC) + all its methods across 17K+ LOC, instead of a focused 20-line `UserReader` interface.
- **Mock Generation Blocked**: The `store/mock/generate.go` exists but `mock_store.go` has not been generated (0 bytes). AI cannot write unit tests without manually stubbing the entire store.
- **Incorrect Code Synthesis**: When AI sees `s.store.GetUser(...)`, it cannot determine the method signature without loading `store/principal.go` (839 LOC). With an interface, the contract would be immediately visible.

## Evidence

```go
// backend/api/v1/acl.go — uses concrete store
type ACLInterceptor struct {
    store      *store.Store    // ← concrete dependency
    secret     string
    iamManager *iam.Manager
    profile    *config.Profile
}

// backend/store/interfaces.go — interfaces exist but unused by services
type UserReader interface {
    GetUser(ctx context.Context, find *FindUserMessage) (*UserMessage, error)
    ListUsers(ctx context.Context, find *FindUserMessage) ([]*UserMessage, error)
}
```

```
# Files importing *store.Store directly:
$ grep -rl "store\.Store\b" backend/ --include="*.go" | wc -l
76
```

## Recommended Remediation

1. **Inject Interfaces**: Refactor service constructors to accept granular interfaces:
   ```go
   // Before
   func NewAuthService(store *store.Store, ...) *AuthService
   
   // After  
   func NewAuthService(users store.UserStore, settings store.SettingReader, ...) *AuthService
   ```

2. **Generate Mocks**: Run `go generate ./backend/store/mock/...` to produce `mock_store.go` from the existing interface definitions.

3. **Incremental Migration**: Start with `auth_service.go` and `sql_service.go` as they have the highest fan-out to store methods.

## Files to Modify

```
backend/api/v1/auth_service.go
backend/api/v1/sql_service.go
backend/api/v1/database_service.go
backend/api/v1/rollout_service.go
backend/api/v1/acl.go
backend/store/mock/generate.go → run go generate
```

## Dependencies

- Blocked by: None (interfaces already defined)
- Enables: AI-BLOCKER-006 (unit test coverage)
