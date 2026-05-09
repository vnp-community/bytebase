# Solution: Service Layer Modularization — CR-WEAK-004

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-WEAK-004                                             |
| **CR Reference**   | CR-WEAK-004                                              |
| **Title**          | Service File Decomposition Strategy                      |
| **Affected Layers**| L4 (Service)                                             |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §5 (L4): 79 files in `backend/api/v1/`, ~1MB+ total. Services follow ConnectRPC handler pattern — each service struct implements generated interface methods.

Per TDD.md §3.1: Interceptor chain (Validate → Auth → ACL → Audit) is applied uniformly via ConnectRPC interceptors — not per-service. Services are stateless handlers that delegate to L5 components and L8 store.

**Key insight**: Go allows methods on a struct to be defined across multiple files in the same package. Splitting methods into separate files is a **zero-impact refactor** — no API changes, no import changes.

---

## 2. Current File Sizes (verified)

| File                  | Lines | Concern Mix                              |
|-----------------------|-------|------------------------------------------|
| `auth_service.go`     | 1930  | Login + Signup + OAuth + MFA + Password + Email |
| `sql_service.go`      | 1876  | Query + Check + AI + Export + Convert    |
| `rollout_service.go`  | 1278  | Rollout CRUD + Task creation + Execution |

---

## 3. Solution Design

### 3.1 Split Strategy — Same Package, Method Distribution

```
Principle: Move method implementations to domain files.
           Keep service struct + constructor in main file.
           All files remain in package v1 (backend/api/v1/).
```

### 3.2 auth_service.go Split Plan

**Current** `auth_service.go` (1930 lines) → **6 files**:

```
backend/api/v1/
├── auth_service.go          # AuthService struct, NewAuthService(), Login()
├── auth_signup.go           # Signup, CreateUser, email verification
├── auth_sso.go              # OAuth2 callback, OIDC/SAML/LDAP flows
├── auth_mfa.go              # TOTP setup, MFA verify, recovery codes
├── auth_password.go         # Password reset, change, policy validation
└── auth_token.go            # JWT generation, refresh, revoke, cookie mgmt
```

**Implementation pattern**:

```go
// auth_service.go — keeps struct + constructor + core Login
package v1

type AuthService struct {
    store          *store.Store
    licenseService *enterprise.LicenseService
    iamManager     *iam.Manager
    secret         string
    profile        *config.Profile
}

func NewAuthService(/*...*/) *AuthService { /*...*/ }
func (s *AuthService) Login(ctx context.Context, req *connect.Request[v1pb.LoginRequest]) (*connect.Response[v1pb.LoginResponse], error) { /*...*/ }
```

```go
// auth_sso.go — SSO methods on same struct
package v1

func (s *AuthService) ExchangeToken(ctx context.Context, req *connect.Request[v1pb.ExchangeTokenRequest]) (*connect.Response[v1pb.ExchangeTokenResponse], error) { /*...*/ }
func (s *AuthService) GetIdentityProviderLoginURL(/*...*/) { /*...*/ }
```

### 3.3 sql_service.go Split Plan

**Current** `sql_service.go` (1876 lines) → **5 files**:

```
backend/api/v1/
├── sql_service.go           # SQLService struct, NewSQLService()
├── sql_query.go             # Execute, Query, AdminExecute streaming
├── sql_check.go             # Check (SQL review), advisor integration
├── sql_ai.go                # NL2SQL, AI explanation, suggestion
└── sql_export.go            # Export (CSV, JSON, XLSX), format conversion
```

### 3.4 rollout_service.go Split Plan

```
backend/api/v1/
├── rollout_service.go       # RolloutService struct, CRUD
├── rollout_task.go          # Task/Stage creation from plan specs
├── rollout_execution.go     # RunTask, CancelTask, RetryTask
└── rollout_converter.go     # Proto ↔ Store conversions
```

### 3.5 CI File Size Enforcement

**New script**: `scripts/lint-file-size.sh`

```bash
#!/bin/bash
MAX_LINES=1500
EXIT_CODE=0

for f in $(find backend/api/v1/ -name '*.go' ! -name '*_test.go' ! -name '*.pb.go'); do
    lines=$(wc -l < "$f")
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "ERROR: $f has $lines lines (max: $MAX_LINES)"
        EXIT_CODE=1
    fi
done

exit $EXIT_CODE
```

**CI integration** (`.github/workflows/lint.yml`):
```yaml
- name: Check file sizes
  run: bash scripts/lint-file-size.sh
```

---

## 4. Execution Order

| Step | Files | Risk | Verification |
|------|-------|------|-------------|
| 1 | `auth_service.go` → 6 files | High (most complex) | `go build ./backend/api/v1/...` + all integration tests |
| 2 | `sql_service.go` → 5 files | Medium | `go build` + SQL query tests |
| 3 | `rollout_service.go` → 4 files | Medium | `go build` + rollout tests |
| 4 | Remaining 7 services | Low | `go build` + full test suite |
| 5 | CI lint rule | None | Verify CI catches violations |

**Critical rule**: One service per PR to minimize merge conflicts.

---

## 5. File Change Manifest

| File | Action | Impact |
|------|--------|--------|
| `backend/api/v1/auth_service.go` | MODIFY (shrink) | Keep struct + constructor + Login |
| `backend/api/v1/auth_signup.go` | NEW | Extracted signup methods |
| `backend/api/v1/auth_sso.go` | NEW | Extracted SSO methods |
| `backend/api/v1/auth_mfa.go` | NEW | Extracted MFA methods |
| `backend/api/v1/auth_password.go` | NEW | Extracted password methods |
| `backend/api/v1/auth_token.go` | NEW | Extracted token methods |
| `backend/api/v1/sql_service.go` | MODIFY (shrink) | Keep struct + constructor |
| `backend/api/v1/sql_query.go` | NEW | Query execution methods |
| `backend/api/v1/sql_check.go` | NEW | SQL review methods |
| `backend/api/v1/sql_ai.go` | NEW | AI methods |
| `backend/api/v1/sql_export.go` | NEW | Export methods |
| `scripts/lint-file-size.sh` | NEW | CI enforcement |

## 6. Test Strategy

```bash
# Verify zero functional regression after each split
go build ./backend/api/v1/...
go vet ./backend/api/v1/...
go test ./backend/tests/... -count=1  # integration tests
```

## 7. Rollback

Pure file reorganization — `git revert` restores original files. No DB changes, no API changes.
