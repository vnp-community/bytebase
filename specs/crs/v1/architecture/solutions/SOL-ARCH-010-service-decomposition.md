# Solution: Service Layer Decomposition — CR-ARCH-010

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-010                                             |
| **CR Reference**   | CR-ARCH-010                                              |
| **Title**          | Domain-Based File Split + CI Lint Guard                  |
| **Affected Layers**| L4 (Service)                                             |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §4 (L4 — Service Layer):
- 79 files, 36,812 lines in `backend/api/v1/`
- 6 files exceed 1,000 lines

Per [TDD.md](../../TDD.md) §3.3:
- Service pattern: single struct per gRPC service, all methods in one file

---

## 2. Current State Analysis

```
auth_service.go         1,930 lines  ← 7 domains mixed
sql_service.go          1,876 lines  ← query + check + AI + export
document_masking.go     1,385 lines  ← 4 DB engines mixed
rollout_service.go      1,278 lines  ← CRUD + task management
issue_service.go        1,134 lines  ← issue lifecycle + comments
database_service.go     1,067 lines  ← schema + sync + metadata
```

---

## 3. Solution Design

### 3.1 Split Plan

Same Go package (`package v1`), same struct, different files.

**auth_service.go (1,930 lines) → 4 files**:

```
auth_service.go              → Core struct + constructor (150 lines)
auth_service_login.go        → Login, Logout, CreateUser (450 lines)
auth_service_mfa.go          → MFA setup, verify, backup codes (380 lines)
auth_service_sso.go          → OIDC, SAML, SSO providers (500 lines)
auth_service_password.go     → Password reset, change, hash (450 lines)
```

```go
// auth_service.go — AFTER: just struct + constructor
package v1

type AuthService struct {
    store          *store.Store
    licenseService *enterprise.LicenseService
    profile        *config.Profile
    iamManager     *iam.Manager
    secret         string
}

func NewAuthService(...) *AuthService { ... }
```

```go
// auth_service_login.go — login domain methods
package v1

func (s *AuthService) Login(ctx context.Context, req *v1pb.LoginRequest) (*v1pb.LoginResponse, error) { ... }
func (s *AuthService) Logout(ctx context.Context, req *v1pb.LogoutRequest) (*emptypb.Empty, error) { ... }
func (s *AuthService) CreateUser(ctx context.Context, req *v1pb.CreateUserRequest) (*v1pb.User, error) { ... }
```

**sql_service.go (1,876 lines) → 4 files**:

```
sql_service.go               → Struct + constructor (100 lines)
sql_service_query.go         → Execute, Query, ExportCSV (500 lines)
sql_service_check.go         → SQL Review, syntax check (450 lines)
sql_service_ai.go            → AI completion, chat (400 lines)
sql_service_export.go        → Data export, streaming (426 lines)
```

**document_masking.go (1,385 lines) → 4 files**:

```
document_masking.go          → Interface + dispatcher (200 lines)
document_masking_pg.go       → PostgreSQL masking (350 lines)
document_masking_mysql.go    → MySQL masking (350 lines)
document_masking_mongo.go    → MongoDB + CosmosDB masking (485 lines)
```

**rollout_service.go (1,278 lines) → 3 files**:

```
rollout_service.go           → Struct + constructor (100 lines)
rollout_service_crud.go      → Create, Get, List, Update (550 lines)
rollout_service_task.go      → Task management, status (628 lines)
```

### 3.2 CI Lint Script

**New file**: `scripts/lint-file-size.sh`

```bash
#!/bin/bash
# CI lint: enforce max file size in service layer
# Exit code: number of violations (0 = pass)

MAX_LINES=${1:-800}
TARGET_DIR="backend/api/v1"
ERRORS=0

echo "=== File Size Lint (max: $MAX_LINES lines) ==="

for f in $(find "$TARGET_DIR" -name '*.go' -not -name '*_test.go' | sort); do
    lines=$(wc -l < "$f" | tr -d ' ')
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ $f: $lines lines (exceeds $MAX_LINES)"
        ERRORS=$((ERRORS + 1))
    fi
done

if [ "$ERRORS" -eq 0 ]; then
    echo "✅ All files within limit"
else
    echo ""
    echo "❌ $ERRORS file(s) exceed $MAX_LINES lines"
    echo "   Split large files by domain into same-package files."
fi

exit $ERRORS
```

### 3.3 Git Blame Preservation

Use `git mv` equivalent via careful commit strategy:

```bash
# Step 1: Copy file (keeps content)
cp auth_service.go auth_service_login.go

# Step 2: Edit both files (remove irrelevant methods from each)
# Step 3: Commit with descriptive message
git add -A
git commit -m "refactor(api): split auth_service.go by domain

Split auth_service.go (1,930 lines) into domain-specific files:
- auth_service.go: core struct + constructor
- auth_service_login.go: login/logout/createUser
- auth_service_mfa.go: MFA lifecycle
- auth_service_sso.go: SSO providers
- auth_service_password.go: password management

No behavioral changes. Same package, same struct."
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/api/v1/auth_service_login.go` | L4 | **NEW** | Login methods |
| `backend/api/v1/auth_service_mfa.go` | L4 | **NEW** | MFA methods |
| `backend/api/v1/auth_service_sso.go` | L4 | **NEW** | SSO methods |
| `backend/api/v1/auth_service_password.go` | L4 | **NEW** | Password methods |
| `backend/api/v1/auth_service.go` | L4 | **MODIFY** | Only struct + constructor |
| `backend/api/v1/sql_service_query.go` | L4 | **NEW** | Query methods |
| `backend/api/v1/sql_service_check.go` | L4 | **NEW** | SQL Review |
| `backend/api/v1/sql_service_ai.go` | L4 | **NEW** | AI methods |
| `backend/api/v1/sql_service_export.go` | L4 | **NEW** | Export methods |
| `backend/api/v1/sql_service.go` | L4 | **MODIFY** | Only struct |
| `scripts/lint-file-size.sh` | CI | **NEW** | Enforcement |

---

## 5. Key Invariants

1. **Same package** — `package v1` in all files
2. **Same struct** — methods stay on `AuthService`, `SQLService`, etc.
3. **Zero import changes** — no consumer changes required
4. **No behavioral changes** — pure code movement

---

## 6. Rollback Plan

`git revert` the split commit → single file restored.
