# Solution: Policy Lifecycle & GitOps Pipeline

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-007 |
| **CR Reference** | CR-POL-007 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-004, SOL-POL-005 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| Lifecycle Manager | **L5 — Component** | Shared lifecycle state machine |
| GitOps Sync | **L5 — Component** | Policy repo sync logic |
| Test Runner | **L5 — Component** | Policy test execution |
| Impact Analyzer | **L5 — Component** | Pre-deployment analysis |
| Version Manager | **L8 — Store** | Policy version history persistence |
| Webhook Handler | **L2 — API Gateway** | Git webhook receiver endpoint |
| Policy Version table | **L8 — Store** | New table for version history |

---

## 2. Package Structure

```
backend/component/policy/lifecycle/
├── manager.go         ← LifecycleManager: state machine + transition rules
├── gitops.go          ← GitOpsSync: Git repository watching + sync
├── test_runner.go     ← PolicyTestRunner: OPA test, Conftest, custom fixtures
├── impact.go          ← ImpactAnalyzer: pre-deployment impact analysis
├── version.go         ← VersionManager: version history + rollback

backend/api/v1/
└── policy_webhook.go  ← Git webhook handler (GitHub/GitLab/Bitbucket)

backend/store/
└── policy_version.go  ← CRUD for policy_version table
```

---

## 3. Key Design Decisions

### 3.1 Policy Lifecycle State Machine

```
DRAFT ──► TESTING ──► STAGING ──► ACTIVE ──► DEPRECATED ──► ARCHIVED
  ▲                      │          │                │
  └──────────────────────┘          │                │
         (rejected)                 ▼                ▼
                               ROLLBACK          (auto after TTL)
```

```go
type LifecycleManager struct {
    store          *store.Store
    compiler       *PolicyCompiler     // SOL-POL-005
    policyManager  *PolicyManager      // SOL-POL-001
    testRunner     *PolicyTestRunner
    impactAnalyzer *ImpactAnalyzer
}

// Transition validates and executes state transitions.
func (m *LifecycleManager) Transition(ctx, policyID string, targetState PolicyStatus) error {
    // 1. Validate transition is allowed
    // 2. Execute transition actions:
    //    DRAFT → TESTING:   Run automated tests
    //    TESTING → STAGING: Enable shadow evaluation
    //    STAGING → ACTIVE:  Full enforcement, create version snapshot
    //    ACTIVE → DEPRECATED: Set phase-out TTL, warn on match
    //    * → ROLLBACK: Activate previous version
}

// Valid transitions
var validTransitions = map[PolicyStatus][]PolicyStatus{
    StatusDraft:      {StatusTesting},
    StatusTesting:    {StatusStaging, StatusDraft},      // Can reject back to DRAFT
    StatusStaging:    {StatusActive, StatusDraft},        // Can reject back to DRAFT
    StatusActive:     {StatusDeprecated},
    StatusDeprecated: {StatusArchived, StatusActive},     // Can re-activate
}
```

### 3.2 GitOps Workflow Integration

```
Developer          Git Repository         CI/CD Pipeline        Bytebase
    │                    │                      │                    │
    ├─ Push branch ─────►│                      │                    │
    │                    ├─ Webhook ────────────►│                    │
    │                    │                      ├─ POST /api/v1/     │
    │                    │                      │  policy-webhook ──►│
    │                    │                      │                    ├─ Lint (Regal)
    │                    │                      │                    ├─ Validate
    │                    │                      │                    ├─ Test
    │                    │                      │                    ├─ Impact analysis
    │                    │                      │◄── Status ─────────┤
    │                    │  ◄── PR status ──────┤                    │
    ├─ Review + Merge ──►│                      │                    │
    │                    ├─ Merge webhook ──────►│                    │
    │                    │                      ├─ Deploy ──────────►│
    │                    │                      │                    ├─ STAGING (shadow)
    │                    │                      │                    ├─ Monitor 24h
    │                    │                      │                    └─ Promote → ACTIVE
```

```go
// backend/api/v1/policy_webhook.go
// New echo route: POST /api/v1/policy-webhook
func (s *Server) handlePolicyWebhook(c echo.Context) error {
    // 1. Verify webhook signature (GitHub/GitLab HMAC)
    // 2. Parse event (push, PR, merge)
    // 3. On PR: lint + validate + test + impact → return status
    // 4. On merge to main: trigger OPAL sync (SOL-POL-004)
    // 5. On tag: promote STAGING → ACTIVE
}
```

**Route registration** in `echo_routes.go`:

```go
// New webhook route alongside existing /hook/scim/* and /hook/stripe/*
e.POST("/hook/policy-gitops", s.handlePolicyWebhook)
```

### 3.3 Policy Test Runner

```go
type PolicyTestRunner struct {
    compiler *PolicyCompiler        // SOL-POL-005
    engines  map[string]PolicyEngine // SOL-POL-001
}

func (r *PolicyTestRunner) RunTests(policy *PolicyDefinition) (*TestReport, error) {
    switch policy.Language {
    case PolicyLanguageRego:
        return r.runOPATests(policy)    // OPA test framework (*_test.rego)
    case PolicyLanguageCedar:
        return r.runCedarTests(policy)  // Cedar validation + simulation
    default:
        return r.runFixtureTests(policy) // JSON input → expected decision
    }
}

type TestReport struct {
    TotalTests  int
    Passed      int
    Failed      int
    Skipped     int
    Results     []*TestResult
    Duration    time.Duration
}
```

### 3.4 Impact Analysis

```go
type ImpactAnalyzer struct {
    store         *store.Store
    policyManager *PolicyManager
}

func (a *ImpactAnalyzer) Analyze(ctx, policy *PolicyDefinition) (*ImpactAnalysis, error) {
    // 1. Count affected resources (databases, instances matching scope)
    // 2. Count affected users (principals matching subject patterns)
    // 3. Sample decision diffs: evaluate 100 recent requests with old vs new policy
    // 4. Detect conflicts with existing active policies (Cedar analyzer if available)
    return &ImpactAnalysis{
        AffectedResources: resourceCount,
        AffectedUsers:     userCount,
        DecisionChanges:   diffCount,
        Conflicts:         conflictCount,
        Samples:           samples,
    }, nil
}
```

### 3.5 Policy Versioning

```sql
-- New table: policy_version
CREATE TABLE policy_version (
    id BIGSERIAL,
    workspace TEXT NOT NULL,
    policy_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    source TEXT NOT NULL,           -- Policy source code snapshot
    compiled BYTEA,
    status TEXT NOT NULL,           -- Status at time of version
    author TEXT NOT NULL,
    git_ref TEXT NOT NULL DEFAULT '',
    change_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace, id),
    FOREIGN KEY (workspace, policy_id) REFERENCES policy_definition(workspace, id)
);

CREATE INDEX idx_policy_version_policy ON policy_version(workspace, policy_id, version DESC);
```

```go
type VersionManager struct {
    store *store.Store
}

// CreateVersion snapshots current policy state.
func (v *VersionManager) CreateVersion(ctx, policy *PolicyDefinition) (int, error)

// Rollback activates a previous version.
func (v *VersionManager) Rollback(ctx, policyID string, targetVersion int) error {
    // 1. Get version snapshot
    // 2. Update policy_definition with snapshot source/compiled
    // 3. Reload policy in engine
    // 4. Create new version entry marking rollback
}

// ListVersions returns version history for a policy.
func (v *VersionManager) ListVersions(ctx, policyID string) ([]*PolicyVersion, error)
```

---

## 4. Shadow Evaluation (STAGING Mode)

When a policy is in STAGING status, it evaluates but does **not enforce**:

```go
// In PolicyManager.Evaluate():
if policy.Status == StatusStaging {
    // Evaluate and log decision, but don't block request
    decision, _ := engine.Evaluate(ctx, req)
    bus.PolicyDecisionChan <- PolicyDecisionLog{
        PolicyID: policy.ID,
        Allowed:  decision.Allowed,
        Shadow:   true,  // Mark as shadow evaluation
    }
    // Return default allow — don't affect actual request
    return &PolicyDecision{Allowed: true, Reason: "shadow mode"}, nil
}
```

This enables **safe rollout**: observe policy behavior for 24-48h before promoting to ACTIVE.

---

## 5. Integration with Existing Systems

| System | Integration |
|---|---|
| Existing OrgPolicyService | Internal policies (MASKING, ACCESS) continue through CELEngine |
| OPAL (SOL-POL-004) | Git merge → OPAL sync → auto-distribute to all instances |
| Compiler (SOL-POL-005) | Lint + validate + test in CI/CD webhook handler |
| DataCleaner runner | Cleanup archived policy versions (configurable retention) |
| AuditInterceptor | Shadow evaluation decisions logged to audit_log |

---

## 6. Risk & Mitigation

| Risk | Mitigation |
|---|---|
| GitOps webhook abuse | HMAC signature verification, rate limiting |
| Shadow mode performance | Shadow evaluations have dedicated budget (< 5% CPU overhead) |
| Rollback to incompatible version | Validate rollback version against current schema before activation |
| Impact analysis accuracy | Sample-based (100 recent requests), clearly labeled as estimate |
