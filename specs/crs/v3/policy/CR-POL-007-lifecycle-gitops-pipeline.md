# Change Request: Policy Lifecycle & GitOps Pipeline

| Field | Value |
|---|---|
| **CR ID** | CR-POL-007 |
| **Title** | Policy Lifecycle & GitOps Pipeline |
| **Plan** | ENTERPRISE |
| **Priority** | P2 — Medium |
| **Status** | Draft |
| **Created** | 2026-05-17 |
| **Author** | VNP AI Ops Team |
| **Dependencies** | CR-POL-004, CR-POL-005 |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế **Policy Lifecycle & GitOps Pipeline** — quản lý vòng đời chính sách từ authoring, testing, staging, deployment, monitoring, đến deprecation. Tích hợp GitOps workflow cho phép policy-as-code từ Git repositories với CI/CD validation pipeline.

### 1.2 Mục tiêu
- Policy lifecycle states: DRAFT → TESTING → STAGING → ACTIVE → DEPRECATED → ARCHIVED
- Git-based policy authoring với PR/MR review workflow
- Automated policy testing (Conftest, OPA test, Cedar validation)
- Staged rollout (canary → full deployment)
- Policy versioning với rollback capability
- Policy impact analysis trước deployment

---

## 2. Yêu cầu chức năng

### FR-001: Policy Lifecycle State Machine

```
DRAFT ──► TESTING ──► STAGING ──► ACTIVE ──► DEPRECATED ──► ARCHIVED
  ▲                      │          │                │
  └──────────────────────┘          │                │
         (rejected)                 ▼                ▼
                               ROLLBACK          (auto after TTL)
```

- **DRAFT**: Policy đang được soạn, chưa evaluate
- **TESTING**: Chạy automated tests (Conftest, OPA test)
- **STAGING**: Shadow mode — evaluate nhưng không enforce, log decisions
- **ACTIVE**: Full enforcement
- **DEPRECATED**: Warning khi match, phase-out period
- **ARCHIVED**: Removed from evaluation, retained for audit

### FR-002: GitOps Workflow

```
Developer                   Git                    CI/CD                  Bytebase
    │                        │                       │                       │
    ├── Create policy ──────►│                       │                       │
    │   (new branch)         │                       │                       │
    │                        ├── PR webhook ────────►│                       │
    │                        │                       ├── Lint (Regal) ──────►│
    │                        │                       ├── Test (OPA test) ───►│
    │                        │                       ├── Validate ──────────►│
    │                        │                       ├── Impact analysis ───►│
    │                        │                       │◄──── Results ─────────┤
    │                        │   ◄── Status ─────────┤                       │
    ├── Review + Approve ───►│                       │                       │
    │                        ├── Merge to main ─────►│                       │
    │                        │                       ├── Deploy (OPAL) ─────►│
    │                        │                       │                       ├─ STAGING
    │                        │                       │                       ├─ Shadow eval
    │                        │                       │                       ├─ Promote
    │                        │                       │                       └─ ACTIVE
```

### FR-003: Policy Testing Integration

```go
type PolicyTestRunner struct {
    compiler *PolicyCompiler
    engines  map[string]PolicyEngine
}

// RunTests executes test suites for a policy.
func (r *PolicyTestRunner) RunTests(policy *PolicyDefinition) (*TestReport, error)

type TestReport struct {
    TotalTests  int
    Passed      int
    Failed      int
    Skipped     int
    Results     []*TestResult
    Duration    time.Duration
}
```

Support:
- OPA test framework (`*_test.rego`)
- Conftest-style tests
- Custom test fixtures (JSON input → expected decision)

### FR-004: Policy Impact Analysis

Trước khi deploy policy mới, phân tích ảnh hưởng:

```go
type ImpactAnalysis struct {
    AffectedResources  int      // How many resources this policy covers
    AffectedUsers      int      // How many users would be impacted
    DecisionChanges    int      // How many existing decisions would change
    Conflicts          int      // Conflicts with existing policies
    Samples            []*ImpactSample  // Sample decision diffs
}
```

### FR-005: Policy Versioning & Rollback

- Mỗi policy change tạo new version
- Version history lưu trong `policy_definition` table
- Rollback = activate previous version
- Git tag cho mỗi deployed version

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| Lifecycle Manager | `backend/component/policy/lifecycle/manager.go` | New |
| GitOps Sync | `backend/component/policy/lifecycle/gitops.go` | New |
| Test Runner | `backend/component/policy/lifecycle/test_runner.go` | New |
| Impact Analyzer | `backend/component/policy/lifecycle/impact.go` | New |
| Version Manager | `backend/component/policy/lifecycle/version.go` | New |
| Webhook Handler | `backend/api/v1/policy_webhook.go` | New: Git webhook receiver |
| Store: policy_version | `backend/store/policy_version.go` | New: version history table |

---

## 4. Test Cases

| Test ID | Mô tả | Expected Result |
|---|---|---|
| TC-001 | Policy state transition DRAFT → TESTING | Tests executed |
| TC-002 | Policy state transition TESTING → STAGING | Shadow evaluation started |
| TC-003 | STAGING → ACTIVE promotion | Full enforcement enabled |
| TC-004 | Rollback to previous version | Previous policy active |
| TC-005 | Git push triggers policy sync | Policy updated in Bytebase |
| TC-006 | Impact analysis before deploy | Affected resources/users counted |
| TC-007 | OPA test suite execution | Pass/fail report generated |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | Lifecycle state machine | Sprint 1 |
| Phase 2 | Policy versioning + rollback | Sprint 1-2 |
| Phase 3 | Test runner integration | Sprint 2-3 |
| Phase 4 | GitOps sync + webhook | Sprint 3-4 |
| Phase 5 | Impact analysis | Sprint 4-5 |
| Phase 6 | UI for lifecycle management | Sprint 5-6 |
