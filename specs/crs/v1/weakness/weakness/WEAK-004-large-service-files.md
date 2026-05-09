# WEAK-004 — Large Service Files — Maintainability Risk

| Metadata       | Value                                      |
|----------------|--------------------------------------------|
| ID             | WEAK-004                                   |
| Category       | Code Quality / Maintainability             |
| Severity       | MEDIUM                                     |
| Affected Layer | L4 (Service)                               |
| Source Files   | `backend/api/v1/`                          |

---

## Mô tả

Service layer chứa nhiều files rất lớn (>30KB), vi phạm Single Responsibility Principle.

## Chi tiết

### Largest Service Files

| File                         | Size  | Concern                           |
|-----------------------------|-------|-----------------------------------|
| `rollout_service.go` + deps | ~82KB | Rollout lifecycle, task creation  |
| `auth_service.go`           | 78KB  | Login, SSO, MFA, signup, password |
| `sql_service.go` + deps     | 77KB  | Query, check, AI, converter      |
| `setting_service.go` + conv | 68KB  | 30+ setting types, validation    |
| `instance_service.go` + conv| 64KB  | Instance CRUD, sync, test conn   |
| `issue_service.go` + conv   | 60KB  | Issue lifecycle, search, comment |
| `project_service.go` + conv | 56KB  | Project CRUD, members, VCS       |
| `database_service.go` + deps| 57KB  | DB metadata, changelog, sync     |
| `release_service.go` + deps | 56KB  | Release mgmt + AI lint           |
| `document_masking.go`       | 44KB  | NoSQL document masking           |
| `acl.go`                    | 19KB  | ACL interceptor (single file)    |
| `audit.go`                  | 25KB  | Audit interceptor (single file)  |

### Problems

1. **auth_service.go (78KB)** — mixes login, signup, OAuth, MFA, password policy, email verification trong cùng file.
2. **sql_service.go (77KB)** — combines query execution, SQL checking, AI features, format conversion.
3. **setting_service.go (68KB)** — handles 30+ different setting types with different validation logic.

### Code Smell Indicators

- 50+ `nolint` directives across service layer.
- Complex switch statements spanning 100+ lines.
- Converter files (e.g., `database_converter.go` at 34KB) doing proto ↔ store mapping.

## Impact

- **Code review difficulty** — 78KB files are hard to review effectively.
- **Merge conflicts** — high frequency when multiple devs modify same file.
- **Testing complexity** — monolithic service functions require extensive mocking.
- **Onboarding** — new developers overwhelmed by file sizes.

## Khuyến nghị

1. Split `auth_service.go` → `auth_login.go`, `auth_sso.go`, `auth_mfa.go`, `auth_password.go`.
2. Split `sql_service.go` → `sql_query.go`, `sql_check.go`, `sql_ai.go`.
3. Extract converter logic into dedicated packages.
4. Enforce max file size lint rule (~500-1000 lines).
