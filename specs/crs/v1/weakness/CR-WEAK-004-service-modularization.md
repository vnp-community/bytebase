# Change Request: Service Layer Modularization

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-004                                              |
| **Weakness ID**    | WEAK-004                                                 |
| **Title**          | Service Layer Modularization — Split Large Service Files |
| **Category**       | Code Quality / Maintainability                           |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | All services — cross-cutting refactor                    |

---

## 1. Tổng quan

### 1.1 Mô tả
Phân tách các service files lớn (>30KB) thành modules nhỏ hơn theo domain responsibility. Giảm merge conflicts, cải thiện code review quality, và tăng test coverage khả thi.

### 1.2 Bối cảnh
12 service files trong `backend/api/v1/` vượt 30KB, lớn nhất là `rollout_service.go` (~82KB) và `auth_service.go` (78KB). Mỗi file mixes nhiều domain concerns — e.g., auth_service handles login, signup, OAuth, MFA, password policy, email verification trong cùng file. Điều này vi phạm Single Responsibility Principle và tạo bottleneck cho parallel development.

### 1.3 Mục tiêu
- Không file service nào > 1500 lines (~40KB)
- Mỗi file có clear single domain responsibility
- Enforce CI lint rule cho max file size
- Zero functional regression

---

## 2. Yêu cầu chức năng

### FR-001: Auth Service Decomposition
- **Hiện tại**: `auth_service.go` (78KB) — monolithic auth
- **Target**:

  | New File                 | Responsibility                      | Est. Size |
  |--------------------------|-------------------------------------|-----------|
  | `auth_service.go`        | Core AuthService struct + Login     | ~15KB     |
  | `auth_signup.go`         | Signup + email verification         | ~12KB     |
  | `auth_sso.go`            | OAuth2 + OIDC + SAML + LDAP flows  | ~20KB     |
  | `auth_mfa.go`            | TOTP 2FA setup + verification      | ~10KB     |
  | `auth_password.go`       | Password policy + reset + change    | ~10KB     |
  | `auth_token.go`          | JWT generation + refresh + revoke   | ~10KB     |

- **Acceptance Criteria**:
  - AC-1: All existing AuthService gRPC methods work identically
  - AC-2: No public API changes (same service + method names)
  - AC-3: Each new file ≤ 1500 lines

### FR-002: SQL Service Decomposition
- **Hiện tại**: `sql_service.go` (77KB) — query + check + AI + convert
- **Target**:

  | New File                 | Responsibility                      | Est. Size |
  |--------------------------|-------------------------------------|-----------|
  | `sql_service.go`         | Core SQLService struct + Execute    | ~15KB     |
  | `sql_query.go`           | Query execution + result streaming  | ~18KB     |
  | `sql_check.go`           | SQL review + advisor integration    | ~15KB     |
  | `sql_ai.go`              | NL2SQL + AI explanation + suggest   | ~12KB     |
  | `sql_export.go`          | Data export (CSV, JSON, XLSX)       | ~10KB     |

- **Acceptance Criteria**: Same as FR-001 (zero API regression, size limits)

### FR-003: Rollout Service Decomposition
- **Hiện tại**: `rollout_service.go` + deps (~82KB)
- **Target**:

  | New File                     | Responsibility                   | Est. Size |
  |------------------------------|----------------------------------|-----------|
  | `rollout_service.go`         | Core + CRUD operations           | ~15KB     |
  | `rollout_task_creation.go`   | Task + stage creation logic      | ~20KB     |
  | `rollout_execution.go`       | Run + cancel + retry logic       | ~15KB     |
  | `rollout_policy.go`          | Rollout policy evaluation        | ~10KB     |
  | `rollout_converter.go`       | Proto ↔ Store converters         | ~15KB     |

### FR-004: Converter Extraction
- **Mô tả**: Extract converter logic into sub-package.
- **Logic**: `backend/api/v1/convert/` package cho shared proto ↔ store mappings
- **Benefits**: Reduces each service file, enables converter unit testing

### FR-005: CI File Size Lint Rule
- **Mô tả**: golangci-lint custom rule hoặc script rejecting files > 1500 lines.
- **Acceptance Criteria**:
  - AC-1: CI fails if any `.go` file in `backend/api/v1/` > 1500 lines
  - AC-2: Generated files (`.pb.go`) excluded from check
  - AC-3: Test files excluded from check

---

## 3. Yêu cầu kỹ thuật

### 3.1 Split Strategy

```
Principle: Move methods, NOT restructure interfaces.

1. Keep service struct in main file (e.g., auth_service.go)
2. Move method implementations to domain files
3. All files in same package (backend/api/v1/) — no import changes
4. Converter code → backend/api/v1/convert/ sub-package
```

### 3.2 Files Affected

| Original File              | Size  | Action                               |
|---------------------------|-------|--------------------------------------|
| `auth_service.go`          | 78KB  | Split into 6 files                  |
| `sql_service.go`           | 77KB  | Split into 5 files                  |
| `rollout_service.go`       | 82KB  | Split into 5 files                  |
| `setting_service.go`       | 68KB  | Split into 3 files                  |
| `instance_service.go`      | 64KB  | Split into 3 files                  |
| `issue_service.go`         | 60KB  | Split into 3 files                  |
| `project_service.go`       | 56KB  | Split into 3 files                  |
| `database_service.go`      | 57KB  | Split into 3 files                  |
| `release_service.go`       | 56KB  | Split into 3 files                  |
| `document_masking.go`      | 44KB  | Split into 2 files                  |

### 3.3 Không có Database Changes — pure refactor

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                  |
|------------|---------------------------------------------------------------|----------------------------------|
| TC-001     | All existing gRPC integration tests pass after split         | Zero failures                    |
| TC-002     | auth_service.go ≤ 1500 lines after split                    | File size compliant              |
| TC-003     | CI file size lint check passes                                | No violations                    |
| TC-004     | CI file size lint check catches new large file                | CI fails                         |
| TC-005     | `go vet ./backend/api/v1/...` passes                         | No vet errors                    |
| TC-006     | Converter sub-package compiles independently                  | Clean build                      |

---

## 5. Rollout Plan

| Phase   | Mô tả                                              | Timeline     |
|---------|------------------------------------------------------|--------------|
| Phase 1 | auth_service.go split (highest risk, most complex)  | Sprint 1     |
| Phase 2 | sql_service.go + rollout_service.go split           | Sprint 2     |
| Phase 3 | Remaining 7 large service files                     | Sprint 3     |
| Phase 4 | Converter extraction to sub-package                 | Sprint 3     |
| Phase 5 | CI file size lint enforcement                       | Sprint 4     |

---

## 6. Risks & Mitigations

| Risk                                      | Impact | Mitigation                                    |
|-------------------------------------------|--------|------------------------------------------------|
| Merge conflicts during split              | HIGH   | Do one service at a time, coordinate with team|
| Import cycle after converter extraction   | MEDIUM | Careful dependency analysis before extraction |
| IDE navigation breaks                     | LOW    | Same package — Go tooling handles it          |
| Test file organization confusion          | LOW    | Match test file names to new source files     |
