# Change Request: Test Coverage Hardening

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-WEAK-007                                              |
| **Weakness ID**    | WEAK-007                                                 |
| **Title**          | Test Coverage Hardening — Core Service & Store Layers    |
| **Category**       | Testing / Quality Assurance                              |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | All — cross-cutting quality requirement                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Tăng unit test coverage cho service, store, và component layers từ mức hiện tại (<30%) lên ≥60% qua mock-based testing, fix empty test files, và CI coverage enforcement.

### 1.2 Bối cảnh
- `auth_service.go` (78KB) — **không có unit test file**
- `changelog_test.go` — **14 bytes (empty test file)**
- Integration tests yêu cầu Docker testcontainers → chậm, không portable
- Component layer (bus, webhook, masker) — không có test files
- IAM manager_test.go — chỉ 2.7KB (basic tests)

### 1.3 Mục tiêu
- ≥60% coverage cho service layer (`backend/api/v1/`)
- ≥70% coverage cho store layer (`backend/store/`)
- ≥80% coverage cho component layer (`backend/component/`)
- Fix all empty/placeholder test files
- CI gate rejecting coverage drops

---

## 2. Yêu cầu chức năng

### FR-001: Service Layer Mock Infrastructure
- **Mô tả**: Create mock interfaces cho Store, LicenseService, DBFactory, Bus để enable unit testing service handlers.
- **Target files**:

  | Mock Interface       | Mocks For                        |
  |----------------------|----------------------------------|
  | `mock_store.go`      | Store CRUD methods               |
  | `mock_license.go`    | LicenseService feature checks    |
  | `mock_dbfactory.go`  | DBFactory driver creation        |
  | `mock_bus.go`        | Bus channel operations           |
  | `mock_iam.go`        | IAM Manager permission checks    |

- **Acceptance Criteria**:
  - AC-1: Mocks generated using `go generate` + mockgen or hand-written
  - AC-2: Mocks support error injection for negative path testing
  - AC-3: Mocks reusable across all service test files

### FR-002: Auth Service Unit Tests
- **Mô tả**: Viết unit tests cho auth_service critical paths.
- **Coverage targets**:

  | Method               | Test Scenarios                                | Count |
  |----------------------|-----------------------------------------------|-------|
  | Login                | Success, wrong password, disabled user, locked| 4     |
  | Signup               | Success, duplicate email, weak password       | 3     |
  | OAuth callback       | Success, invalid state, provider error        | 3     |
  | MFA verify           | Success, wrong code, expired code             | 3     |
  | Password reset       | Success, expired token, invalid token         | 3     |
  | Token refresh        | Success, expired refresh, revoked token       | 3     |

- **Acceptance Criteria**:
  - AC-1: ≥60% coverage cho auth_service.go
  - AC-2: All error paths tested (mock error injection)
  - AC-3: Tests run without Docker/testcontainers

### FR-003: Store Layer Test Completion
- **Mô tả**: Fix empty test files và add missing tests.
- **Priority files**:

  | File                      | Current State | Target                           |
  |---------------------------|---------------|----------------------------------|
  | `changelog_test.go`       | Empty (14B)   | CRUD + query filter tests        |
  | `plan_test.go`            | Missing       | Create, Get, List, Update tests  |
  | `issue_test.go`           | Missing       | Create, Get, List, Search tests  |
  | `task_test.go`            | Missing       | Create, Get, Update status tests |
  | `policy_test.go`          | Missing       | CRUD + merge logic tests         |

- **Acceptance Criteria**:
  - AC-1: No empty test files in backend/store/
  - AC-2: Each store entity has ≥ 5 unit tests
  - AC-3: ≥70% store layer coverage

### FR-004: Component Layer Tests
- **Mô tả**: Add tests cho untested components.
- **Priority**:

  | Component       | File                              | Test Focus                      |
  |-----------------|-----------------------------------|---------------------------------|
  | IAM Manager     | `component/iam/manager_test.go`   | Expand error path tests         |
  | Bus             | `component/bus/bus_test.go`       | Channel send/receive, overflow  |
  | Masker          | `component/masker/masker_test.go` | Masking correctness per type    |
  | Secret Manager  | `component/secret/secret_test.go` | Get/Set/Rotate tests            |

- **Acceptance Criteria**:
  - AC-1: ≥80% component layer coverage
  - AC-2: Bus test covers channel overflow behavior
  - AC-3: Masker tests cover all masking types (full, partial, hash)

### FR-005: CI Coverage Gate
- **Mô tả**: CI pipeline rejects PRs that decrease coverage below threshold.
- **Config**:
  ```yaml
  coverage:
    thresholds:
      backend/api/v1/: 60%
      backend/store/: 70%
      backend/component/: 80%
    mode: delta  # Reject if coverage drops from baseline
  ```
- **Acceptance Criteria**:
  - AC-1: CI reports coverage delta per package
  - AC-2: PR blocked if coverage drops below threshold
  - AC-3: Coverage badge on README

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File/Package                          | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Mock Infrastructure    | `backend/api/v1/testutil/`            | Mock store, license, IAM, bus                |
| Auth Tests             | `backend/api/v1/auth_service_test.go` | New comprehensive test file                  |
| Store Tests            | `backend/store/changelog_test.go`     | Replace empty file with real tests           |
| Store Tests            | `backend/store/plan_test.go`          | New test file                                |
| Component Tests        | `backend/component/bus/bus_test.go`   | New test file                                |
| Component Tests        | `backend/component/masker/masker_test.go` | Expand existing tests                   |
| CI Config              | `.github/workflows/test.yml`          | Add coverage thresholds                      |

### 3.2 Không có Database Changes — testing only

---

## 4. Test Cases (Meta — tests for the test infrastructure)

| Test ID    | Mô tả                                                  | Expected Result                     |
|------------|----------------------------------------------------------|--------------------------------------|
| TC-001     | Mock store returns error → service returns error        | Error propagated correctly           |
| TC-002     | Mock license denies feature → service returns 403       | Feature gate enforced                |
| TC-003     | All auth tests pass without Docker                      | Pure unit tests, <10s execution      |
| TC-004     | Coverage report shows ≥60% for api/v1                   | Threshold met                        |
| TC-005     | CI blocks PR with empty test file                        | PR rejected                          |
| TC-006     | Bus overflow test → no goroutine leak                    | Clean shutdown                       |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------|
| Phase 1 | Mock infrastructure + auth service tests           | Sprint 1-2   |
| Phase 2 | Store layer test completion                        | Sprint 2-3   |
| Phase 3 | Component layer tests (bus, masker, secret)        | Sprint 3     |
| Phase 4 | CI coverage gate enforcement                       | Sprint 4     |

---

## 6. Risks & Mitigations

| Risk                                       | Impact | Mitigation                                 |
|--------------------------------------------|--------|--------------------------------------------|
| Mock maintenance overhead                  | MEDIUM | Generate mocks from interfaces             |
| Tests pass but don't catch real bugs       | MEDIUM | Focus on error paths, not just happy paths |
| Coverage metric gaming (trivial tests)     | LOW    | Code review test quality, not just numbers |
| CI gate blocks urgent hotfixes             | LOW    | Allow override with approval               |
