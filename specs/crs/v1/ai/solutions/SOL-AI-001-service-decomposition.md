# Solution: Service & Model File Decomposition — CR-AI-001

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-AI-001                                               |
| **CR Reference**   | CR-AI-001                                                |
| **Title**          | Same-Package Method Distribution Strategy                |
| **Affected Layers**| L4 (Service), L8 (Store Model)                           |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |

---

## 1. Architecture Context

Per architecture.md §5 (L4): 79 files in `backend/api/v1/`, ~1MB+ total. Services follow ConnectRPC handler pattern — each service struct implements generated interface methods.

Per TDD.md §3.1: Interceptor chain (Validate → Auth → ACL → Audit) is applied uniformly via ConnectRPC interceptors — not per-service. Services are stateless handlers that delegate to L5 components and L8 store.

Per TDD.md §1.2: Bytebase is a **modular monolith** — separation of concerns qua Go packages. All service methods share the same package `v1`.

**Key insight**: Go allows methods on a struct to be defined across multiple files in the same package. Splitting is a **zero-impact refactor** — no API changes, no import changes, no behavioral changes.

---

## 2. Solution Design

### 2.1 Split Strategy — Same Package, Method Distribution

```
Principle: Move method implementations to domain files.
           Keep service struct + constructor in main file.
           All files remain in package v1 (backend/api/v1/).
```

### 2.2 auth_service.go Split Plan

**Current** `auth_service.go` (1930 lines) → **6 files**:

```
backend/api/v1/
├── auth_service.go          # AuthService struct, NewAuthService(), Login()
├── auth_service_signup.go   # Signup, CreateUser, email verification
├── auth_service_sso.go      # OAuth2 callback, OIDC/SAML/LDAP flows  
├── auth_service_mfa.go      # TOTP setup, MFA verify, recovery codes
├── auth_service_password.go # Password reset, change, policy validation
└── auth_service_token.go    # JWT generation, refresh, revoke, cookie mgmt
```

**Implementation pattern** (per TDD.md §3.1 — ConnectRPC handler):

```go
// auth_service.go — keeps struct + constructor + core Login (~250 LOC)
package v1

type AuthService struct {
    store          *store.Store
    licenseService *enterprise.LicenseService
    iamManager     *iam.Manager
    secret         string
    profile        *config.Profile
    // ... other fields
}

func NewAuthService(/*...*/) *AuthService { /*...*/ }

// Login stays in main file — it's the primary entry point
func (s *AuthService) Login(ctx context.Context, 
    req *connect.Request[v1pb.LoginRequest],
) (*connect.Response[v1pb.LoginResponse], error) {
    // Core login logic stays here
}

// Logout stays with Login
func (s *AuthService) Logout(ctx context.Context,
    req *connect.Request[v1pb.LogoutRequest],
) (*connect.Response[v1pb.LogoutResponse], error) {
    // ...
}
```

```go
// auth_service_sso.go — SSO methods on same struct (~350 LOC)
package v1

// ExchangeToken handles OAuth2/OIDC token exchange.
// Architecture: L4 (Service) → L7 (IDP Plugin) → L8 (Store)
// Per TDD.md §6.3: IDP plugin supports OIDC, SAML, LDAP
func (s *AuthService) ExchangeToken(ctx context.Context,
    req *connect.Request[v1pb.ExchangeTokenRequest],
) (*connect.Response[v1pb.ExchangeTokenResponse], error) {
    // SSO logic
}

func (s *AuthService) GetIdentityProviderLoginURL(/*...*/) { /*...*/ }
```

```go
// auth_service_mfa.go — MFA methods (~200 LOC)
package v1

// Per PRD SEC-12: Two-Factor Authentication (TOTP)
func (s *AuthService) CreateMFASetup(/*...*/) { /*...*/ }
func (s *AuthService) VerifyMFA(/*...*/) { /*...*/ }
```

### 2.3 sql_service.go Split Plan

**Current** `sql_service.go` (1876 lines) → **5 files**:

```
backend/api/v1/
├── sql_service.go           # SQLService struct, NewSQLService() (~200 LOC)
├── sql_service_query.go     # Execute, Query (~400 LOC)
├── sql_service_admin.go     # AdminExecute bidi-stream (~350 LOC)
├── sql_service_check.go     # Check (SQL review), advisor integration (~300 LOC)
└── sql_service_export.go    # Export (CSV, JSON, XLSX) (~300 LOC)
```

**AdminExecute bidi-stream** (per TDD.md §3.2):
```go
// sql_service_admin.go
// AdminExecute uses WebSocket proxy via wsproxy (TDD.md §3.2)
// Architecture: L4 → L5 (DBFactory) → L7 (DB Driver.QueryConn)
func (s *SQLService) AdminExecute(ctx context.Context,
    stream *connect.BidiStream[v1pb.AdminExecuteRequest, v1pb.AdminExecuteResponse],
) error {
    // Bidi-stream logic — must handle context cancellation carefully
    // Per TDD.md §5.3: driverCtx can cancel independently from ctx
}
```

### 2.4 rollout_service.go Split Plan

Per architecture.md §7 (L6): TaskRun Scheduler orchestrates pending→running→done lifecycle. Per TDD.md §5.2: Task execution pipeline flows Plan → PlanCheck → Approval → Rollout → TaskRun.

```
backend/api/v1/
├── rollout_service.go          # RolloutService struct, CRUD (~400 LOC)
├── rollout_service_task.go     # Task/Stage creation from plan specs (~400 LOC)
├── rollout_service_execute.go  # RunTask, CancelTask, RetryTask (~300 LOC)
└── rollout_service_convert.go  # Proto ↔ Store conversions (~200 LOC)
```

### 2.5 DCM Workflow Services Split Plan

| Service | Files | LOC Target |
|---------|-------|------------|
| `plan_service.go` (1259) | `plan_service.go` + `plan_service_spec.go` | ≤500 each |
| `issue_service.go` (1242) | `issue_service.go` + `issue_service_lifecycle.go` | ≤500 each |
| `project_service.go` (1275) | `project_service.go` + `project_service_iam.go` | ≤500 each |
| `database_service.go` (1247) | `database_service.go` + `database_service_sync.go` | ≤500 each |
| `instance_service.go` (1181) | `instance_service.go` + `instance_service_activation.go` | ≤500 each |

### 2.6 Store Model Split Plan

Per architecture.md §9 (L8): Store has 74 files + model/. Per TDD.md §4: Store manages PostgreSQL persistence with LRU caches.

```
backend/store/model/
├── database_metadata.go     # DatabaseMetadata struct + FindDatabase*() (~300 LOC)
├── schema_metadata.go       # SchemaMetadata + table/view/sequence lookups (~350 LOC)
├── table_metadata.go        # TableMetadata + column/index operations (~300 LOC)
└── ddl_operations.go        # Create/Drop/Rename types for all objects (~300 LOC)
```

### 2.7 Naming Convention

All extracted files follow pattern: `{service_name}_{domain}.go`
- Prefix matches original: `auth_service_` for all auth files
- This ensures alphabetical grouping in file explorers and AI tool listings

---

## 3. Execution Order

| Step | Files | Risk | Verification |
|------|-------|------|-------------|
| 1 | `auth_service.go` → 6 files | High (most complex, security-critical) | `go build ./backend/api/v1/...` + integration tests |
| 2 | `sql_service.go` → 5 files | Medium (bidi-stream complexity) | `go build` + SQL query tests |
| 3 | `rollout_service.go` → 4 files | Medium (state machine) | `go build` + rollout tests |
| 4 | DCM services (plan, issue, project) | Low | `go build` + full test suite |
| 5 | Infrastructure services (database, instance) | Low | `go build` + sync tests |
| 6 | `store/model/database.go` → 4 files | Low | `go test ./backend/store/model/...` |
| 7 | CI lint rule | None | Verify CI catches violations |

**Critical rule**: One service per PR to minimize merge conflicts.

---

## 4. CI File Size Enforcement

**New script**: `scripts/lint-file-size.sh`

```bash
#!/bin/bash
# Per CR-AI-001: Enforce max 500 LOC for service files, 400 for model files
MAX_SERVICE_LINES=500
MAX_MODEL_LINES=400
EXIT_CODE=0

for f in $(find backend/api/v1/ -name '*.go' ! -name '*_test.go' ! -name '*.pb.go'); do
    lines=$(wc -l < "$f")
    if [ "$lines" -gt "$MAX_SERVICE_LINES" ]; then
        echo "ERROR: $f has $lines lines (max: $MAX_SERVICE_LINES)"
        EXIT_CODE=1
    fi
done

for f in $(find backend/store/model/ -name '*.go' ! -name '*_test.go'); do
    lines=$(wc -l < "$f")
    if [ "$lines" -gt "$MAX_MODEL_LINES" ]; then
        echo "ERROR: $f has $lines lines (max: $MAX_MODEL_LINES)"
        EXIT_CODE=1
    fi
done

exit $EXIT_CODE
```

---

## 5. File Change Manifest

| File | Action | Target LOC |
|------|--------|------------|
| `backend/api/v1/auth_service.go` | MODIFY (shrink) | ≤250 |
| `backend/api/v1/auth_service_signup.go` | NEW | ≤350 |
| `backend/api/v1/auth_service_sso.go` | NEW | ≤350 |
| `backend/api/v1/auth_service_mfa.go` | NEW | ≤200 |
| `backend/api/v1/auth_service_password.go` | NEW | ≤300 |
| `backend/api/v1/auth_service_token.go` | NEW | ≤300 |
| `backend/api/v1/sql_service.go` | MODIFY (shrink) | ≤200 |
| `backend/api/v1/sql_service_query.go` | NEW | ≤400 |
| `backend/api/v1/sql_service_admin.go` | NEW | ≤350 |
| `backend/api/v1/sql_service_check.go` | NEW | ≤300 |
| `backend/api/v1/sql_service_export.go` | NEW | ≤300 |
| `backend/api/v1/rollout_service.go` | MODIFY (shrink) | ≤400 |
| `backend/api/v1/rollout_service_task.go` | NEW | ≤400 |
| `backend/api/v1/rollout_service_execute.go` | NEW | ≤300 |
| `backend/api/v1/rollout_service_convert.go` | NEW | ≤200 |
| `backend/store/model/database_metadata.go` | NEW | ≤300 |
| `backend/store/model/schema_metadata.go` | NEW | ≤350 |
| `backend/store/model/table_metadata.go` | NEW | ≤300 |
| `backend/store/model/ddl_operations.go` | NEW | ≤300 |
| `scripts/lint-file-size.sh` | NEW | CI enforcement |

---

## 6. Layer Compliance Check

Per architecture.md §13 (Dependency Matrix):
- L4 → L5 (Components): ✅ No change — all components still accessed normally
- L4 → L7 (Plugins): ✅ No change — plugin calls stay same
- L4 → L8 (Store): ✅ No change — store methods stay same
- L4 → L9 (Enterprise): ✅ No change — feature gates stay same

**Zero cross-layer impact confirmed.**

---

## 7. Rollback Strategy

Pure file reorganization — `git revert` restores original files. No DB changes, no API changes, no proto changes.
