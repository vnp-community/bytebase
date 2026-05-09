# Change Request: Service Layer Decomposition

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-010                                              |
| **Source ID**      | ARCH-WEAK-005                                            |
| **Title**          | Service Layer Decomposition — Domain-Based File Splitting |
| **Category**       | Architecture (Maintainability)                           |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01 (Change Workflow), SEC-01 (IAM), SQL-01 (SQL Editor) |

---

## 1. Tổng quan

### 1.1 Mô tả
Split service files > 1,000 lines thành domain-focused files trong cùng Go package. `auth_service.go` (1,930 lines) chứa 7 domains (login, signup, MFA, SSO, password, email, token) — split thành 4-5 files.

### 1.2 Bối cảnh
- 79 files, 36,812 lines trong `backend/api/v1/`
- 6 files > 1,000 lines, 18 files > 500 lines
- `auth_service.go` = 1,930 lines (largest)
- `sql_service.go` = 1,876 lines
- Merge conflicts frequent khi 2+ developers edit same large file

### 1.3 Mục tiêu
- Max file size: 800 lines (CI enforced)
- Zero behavioral changes (same package, same struct)
- CI lint script enforcing file size limit
- Reduce merge conflict frequency

---

## 2. Yêu cầu chức năng

### FR-001: Domain-Based File Splitting
- **Mô tả**: Split large service files by domain concern.
- **Target Files**:

  | File | Lines | Split Into |
  |------|-------|------------|
  | `auth_service.go` | 1,930 | `auth_service_login.go`, `auth_service_mfa.go`, `auth_service_sso.go`, `auth_service_token.go` |
  | `sql_service.go` | 1,876 | `sql_service_query.go`, `sql_service_check.go`, `sql_service_ai.go`, `sql_service_export.go` |
  | `document_masking.go` | 1,385 | `document_masking_mongo.go`, `document_masking_cosmos.go`, `document_masking_elastic.go` |
  | `rollout_service.go` | 1,278 | `rollout_service_crud.go`, `rollout_service_task.go` |

- **Acceptance Criteria**:
  - AC-1: All methods stay on same struct (same package refactor)
  - AC-2: No import changes in any consumer
  - AC-3: No behavioral changes — pure code movement
  - AC-4: Each resulting file < 800 lines

### FR-002: CI File Size Lint
- **Mô tả**: CI script blocks PRs that introduce files > 800 lines.
- **Logic**:
  ```bash
  #!/bin/bash
  # scripts/lint-file-size.sh
  MAX_LINES=800
  ERRORS=0
  for f in $(find backend/api/v1/ -name '*.go' -not -name '*_test.go'); do
      lines=$(wc -l < "$f")
      if [ "$lines" -gt "$MAX_LINES" ]; then
          echo "ERROR: $f has $lines lines (max $MAX_LINES)"
          ERRORS=$((ERRORS + 1))
      fi
  done
  exit $ERRORS
  ```
- **Acceptance Criteria**:
  - AC-1: CI runs lint on every PR touching `backend/api/v1/`
  - AC-2: Max lines configurable via CI variable
  - AC-3: Test files excluded from check

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Auth service           | `backend/api/v1/auth_service_*.go`    | Split into 4 files by domain                |
| SQL service            | `backend/api/v1/sql_service_*.go`     | Split into 4 files by domain                |
| Document masking       | `backend/api/v1/document_masking_*.go`| Split into 3 files by engine                |
| Rollout service        | `backend/api/v1/rollout_service_*.go` | Split into 2 files by concern               |
| CI lint                | `scripts/lint-file-size.sh`           | New script                                   |

### 3.2 Database/Frontend Changes
Không có.

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | Build after split → compiles successfully                    | No compilation errors                    |
| TC-002     | All existing integration tests pass                          | Zero regression                          |
| TC-003     | `lint-file-size.sh` → no violations after split              | All files < 800 lines                    |
| TC-004     | New 900-line file → CI lint fails                            | Enforcement working                      |
| TC-005     | Service method location: `grep` finds in expected file       | Predictable file organization            |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Split `auth_service.go` (POC, verify approach)    | Sprint 1     |
| Phase 2 | Split `sql_service.go` + `document_masking.go`     | Sprint 1     |
| Phase 3 | Split `rollout_service.go` + remaining             | Sprint 2     |
| Phase 4 | CI lint script deployment                          | Sprint 2     |

> **Constraint**: Limit 1 service file split per PR to minimize merge conflicts.

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| Merge conflicts during split PR               | HIGH   | One file per PR, coordinate with team               |
| Method ordering confusion                     | LOW    | Group by domain, alphabetical within domain         |
| Git blame history lost                        | LOW    | Use `git log --follow` for history tracking         |
