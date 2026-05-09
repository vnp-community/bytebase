# TASK-AI-002-2: AuthService DI Migration (3 interfaces)

| Field | Value |
|-------|-------|
| Solution | SOL-AI-002 |
| Priority | P0 |
| Depends On | TASK-AI-002-1, TASK-AI-001-1 |
| Est. | M (modify struct + constructor + ~20 method calls) |

## Objective

Replace `*store.Store` in `AuthService` with 3 granular interfaces: `UserStore`, `SettingReader`, `WorkspaceReader`.

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/api/v1/auth_service.go` — struct fields + constructor |
| MODIFY | `backend/api/v1/auth_service_*.go` — `s.store.X()` → `s.users.X()` etc. |
| CREATE | `backend/api/v1/auth_service_test.go` — mock-based unit tests |

## Specification

### Step 1: Struct migration

```go
// BEFORE
type AuthService struct {
    store *store.Store
    // ...
}

// AFTER
type AuthService struct {
    users     store.UserStore
    settings  store.SettingReader
    workspace store.WorkspaceReader
    // ...
}
```

### Step 2: Constructor

```go
func NewAuthService(
    users store.UserStore,
    settings store.SettingReader,
    workspace store.WorkspaceReader,
    // ... remaining params unchanged
) *AuthService
```

### Step 3: Method call migration

Replace all `s.store.GetUser(...)` → `s.users.GetUser(...)`, etc. across all `auth_service_*.go` files.

### Step 4: Unit test (minimum 3 cases)

- `TestAuthService_Login_ValidCredentials`
- `TestAuthService_Login_InvalidCredentials`
- `TestAuthService_Login_MFARequired`

### Verification

```bash
go build ./backend/api/v1/...
go test ./backend/api/v1/... -run TestAuth -count=1
```

## Acceptance Criteria

- [ ] No `*store.Store` reference in AuthService struct
- [ ] All `s.store.` calls replaced with typed interface calls
- [ ] 3+ unit tests using mocks pass
- [ ] `go build` passes
