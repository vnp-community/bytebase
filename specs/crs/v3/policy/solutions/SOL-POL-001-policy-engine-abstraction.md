# Solution: Policy Engine Abstraction Layer

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-001 |
| **CR Reference** | CR-POL-001 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| `PolicyEngine` interface | **L7 — Plugin** | Follows DB Driver plugin pattern (`init()` registration) |
| `PolicyManager` | **L5 — Component** | Shared logic, giống IAM Manager, DBFactory |
| `PolicyDecisionCache` | **L5 — Component** | Follows Store LRU cache pattern (TDD §4.2) |
| `CELEngine` adapter | **L5 — Component** | Wraps existing IAM Manager + OrgPolicyService |
| Store tables | **L8 — Store** | New tables: `policy_engine`, `policy_definition` |
| Proto definitions | **L10 — Infra** | New proto: `policy_engine.proto` |
| Prometheus metrics | **L10 — Infra** | Existing `/metrics` endpoint |

---

## 2. New Package Structure

```
backend/component/policy/
├── engine.go          ← PolicyEngine interface + EngineType enums
├── manager.go         ← PolicyManager orchestrator (multi-engine routing)
├── definition.go      ← PolicyDefinition, PolicyCategory, PolicyScope enums
├── evaluation.go      ← EvaluationRequest, PolicyDecision, PolicyObligation
├── cache.go           ← PolicyDecisionCache (expirable LRU, 4096 entries, 30s TTL)
├── cel_engine.go      ← CELEngine adapter (wraps IAM Manager — backward compat)
├── metrics.go         ← Prometheus: policy_evaluation_total, duration, denied
└── registry.go        ← Engine factory registry (init()-based, like db.Register)
```

---

## 3. Key Design Decisions

### 3.1 Engine Factory Registry — Follows L7 Plugin Pattern

Uses `init()` registration consistent with 22 DB drivers, 9 SQL advisors:

```go
// backend/component/policy/registry.go
func RegisterEngineFactory(name string, factory EngineFactory) {
    factoryMu.Lock()
    defer factoryMu.Unlock()
    factories[name] = factory
}
```

### 3.2 PolicyManager Bootstrap — Step 5.5

```
Server Bootstrap (updated):
  5.  iam.NewManager()
  5.5 policyManager = policy.NewManager(store, iamManager, licenseService, bus)
      ├── Register CELEngine (always — backward compatible)
      ├── Load external engine configs from store (Enterprise only)
      └── Load active PolicyDefinitions into engines
  6.  webhook.NewManager()
```

### 3.3 Decision Cache — Consistent với Store Cache (TDD §4.2)

```go
// Uses hashicorp/golang-lru/v2/expirable — same library as Store
accessCache:  expirable.NewLRU[string, *PolicyDecision](4096, nil, 30*time.Second)
maskingCache: expirable.NewLRU[string, *PolicyDecision](1024, nil, 5*time.Minute)
```

Cache key: `sha256(engine_id | policy_id | subject_type:subject_id | resource_type:resource_id | action)`

### 3.4 CELEngine — Zero Breaking Change Bridge

```go
func (e *CELEngine) Evaluate(ctx, req) (*PolicyDecision, error) {
    // Routes to existing code based on PolicyCategory:
    // ACCESS     → iamManager.CheckPermission()
    // MASKING    → store policy evaluation (existing OrgPolicy logic)
    // GOVERNANCE → existing OrgPolicy check
}
```

### 3.5 Multi-Engine Strategy

```go
func (m *PolicyManager) EvaluateChain(ctx, req, strategy) (*PolicyDecision, error) {
    // ALL_MUST_ALLOW: most restrictive — all engines must allow
    // ANY_ALLOW: permissive — any engine allowing is sufficient
    // PRIORITY_ORDER: evaluate by priority, first deny wins
    // MERGE_OBLIGATIONS: allow if all allow, merge obligations
}
```

---

## 4. Database Schema

```sql
CREATE TABLE policy_engine (
    id TEXT NOT NULL PRIMARY KEY,
    workspace TEXT NOT NULL,
    engine_type TEXT NOT NULL,
    display_name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 100,
    connection_config JSONB NOT NULL DEFAULT '{}',
    strategy TEXT NOT NULL DEFAULT 'PRIORITY_ORDER',
    health_check_interval INTEGER NOT NULL DEFAULT 30,
    health_status TEXT NOT NULL DEFAULT 'UNKNOWN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE policy_definition (
    id TEXT NOT NULL,
    workspace TEXT NOT NULL,
    name TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    category TEXT NOT NULL,
    scope TEXT NOT NULL,
    language TEXT NOT NULL,
    source TEXT NOT NULL,
    compiled BYTEA,
    engine_id TEXT NOT NULL REFERENCES policy_engine(id),
    tags JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'DRAFT',
    author TEXT NOT NULL,
    git_ref TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace, id)
);
```

Uses composite PK `(workspace, id)` consistent with existing store patterns (TDD §4.3).

---

## 5. Migration Path (ACL Interceptor Refactor)

```go
// BEFORE (acl.go, 19KB):
allowed, err := iamManager.CheckPermission(ctx, permission, user, workspaceID, projectIDs...)

// AFTER:
decision, err := policyManager.Evaluate(ctx, &EvaluationRequest{
    Resource: &PolicyResource{Type: "project", ID: projectID},
    Subject:  &PolicySubject{Type: user.Type, ID: user.Email, Roles: roles},
    Action:   permission,
})
// CELEngine internally delegates to iamManager.CheckPermission() — identical behavior
```

---

## 6. Enterprise Feature Gate

```go
// Follows existing pattern (architecture.md §10)
err := licenseService.IsFeatureEnabled(ctx, workspaceID,
    v1pb.PlanFeature_FEATURE_POLICY_ENGINE)
// Not Enterprise → CEL engine only
// Enterprise → load external engines (OPA, Cedar, etc.)
```

---

## 7. Risk & Mitigation

| Risk | Mitigation |
|---|---|
| CELEngine breaks existing IAM | Shadow mode testing, comprehensive unit tests |
| Cache staleness | Short TTL (30s), explicit invalidation on policy update |
| External engine timeout | Configurable timeout + automatic fallback to CEL |
| Performance regression in ACL | Benchmark before/after, cache hit rate monitoring |
