# Solution: Cedar Policy Engine Plugin

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-003 |
| **CR Reference** | CR-POL-003 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-001 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| `CedarEngine` | **L7 — Plugin** | Implements `PolicyEngine`, registered via `init()` |
| `CedarDataLoader` | **L5 — Component** | Syncs Bytebase entities → Cedar entity store |
| `CedarAnalyzer` | **L5 — Component** | Static analysis, conflict detection |
| Cedar Schema | **L7 — Plugin** | Bytebase domain entity/action definitions |
| Cedar Templates | **L7 — Plugin** | Pre-built Cedar policies |

---

## 2. Package Structure

```
backend/component/policy/cedar/
├── init.go            ← RegisterEngineFactory("cedar", ...)
├── engine.go          ← CedarEngine: authorization evaluation
├── config.go          ← CedarConfig struct
├── schema.go          ← Bytebase Cedar schema (entity types, actions)
├── data_loader.go     ← CedarDataLoader: Bytebase store → Cedar entities
├── analyzer.go        ← CedarAnalyzer: conflict detection, coverage analysis
└── templates/
    ├── time_access.cedar       ← Time-based access control
    ├── migration_gov.cedar     ← Production migration governance
    ├── pii_access.cedar        ← PII data access control
    └── destructive_ops.cedar   ← Destructive operation prevention
```

---

## 3. Key Design Decisions

### 3.1 Engine Registration

```go
// backend/component/policy/cedar/init.go
func init() {
    policy.RegisterEngineFactory("cedar", func(config json.RawMessage) (policy.PolicyEngine, error) {
        var cfg CedarConfig
        json.Unmarshal(config, &cfg)
        return NewCedarEngine(&cfg)
    })
}
```

### 3.2 Cedar Engine — Permit/Forbid Evaluation

```go
type CedarEngine struct {
    policySet   *cedar.PolicySet
    schema      *cedar.Schema
    entities    *cedar.EntityStore
    dataLoader  *CedarDataLoader
    mu          sync.RWMutex
}

func (e *CedarEngine) Evaluate(ctx, req) (*PolicyDecision, error) {
    authzReq := cedar.Request{
        Principal: e.buildPrincipal(req.Subject),
        Action:    cedar.EntityUID{Type: "Action", ID: req.Action},
        Resource:  e.buildResource(req.Resource),
        Context:   e.buildContext(req.Context),
    }
    decision, diagnostics := e.policySet.IsAuthorized(e.entities, authzReq)
    return &PolicyDecision{
        Allowed: decision == cedar.Allow,
        Reason:  diagnostics.Reason(),
        Engine:  "cedar",
    }, nil
}
```

### 3.3 Bytebase Cedar Schema

Maps Bytebase domain to Cedar entity hierarchy:

```
Workspace
  ├── Project
  │   └── Database ──── Instance
  │       └── Table       └── Environment
  │           └── Column
  └── User ──── Group ──── Role
      ServiceAccount ──── Role
```

**Entity types**: User, ServiceAccount, Group, Role, Workspace, Project, Environment, Instance, Database, Table, Column

**Actions**: `query`, `migrate`, `export`, `manage`

Each action has typed context attributes (timestamp, hour, sql_type, approval_status, etc.)

### 3.4 Cedar vs OPA — When to Use Cedar

| Use Case | Recommended Engine | Rationale |
|---|---|---|
| Complex data filtering | **OPA** | Rego supports rich data manipulation |
| Authorization (permit/forbid) | **Cedar** | Purpose-built for authz, simpler syntax |
| Formal verification needed | **Cedar** | Built-in conflict detection |
| Compliance auditing | **Cedar** | Policy is human-readable |
| Dynamic context rules | **OPA** | More expressive condition language |
| Cross-system policy reuse | **OPA** | Industry standard, wider adoption |

### 3.5 Conflict Detection — Unique Cedar Feature

```go
type CedarAnalyzer struct {
    policySet *cedar.PolicySet
    schema    *cedar.Schema
}

// DetectConflicts finds policies where permit and forbid overlap.
func (a *CedarAnalyzer) DetectConflicts() ([]PolicyConflict, error)

// ValidateCoverage ensures all defined actions have at least one policy.
func (a *CedarAnalyzer) ValidateCoverage(actions []string) ([]string, error)

// SimulateRequest tests a hypothetical request without enforcement.
func (a *CedarAnalyzer) SimulateRequest(req *EvaluationRequest) (*SimulationResult, error)
```

This enables **proactive policy management** — detect conflicts before deployment (CR-POL-007 integration).

---

## 4. Entity Data Loader

```go
type CedarDataLoader struct {
    store       *store.Store
    entityStore *cedar.EntityStore
}

func (l *CedarDataLoader) SyncAll(ctx) error {
    // Users → User entities with email, department attributes
    // Groups → Group entities with User membership hierarchy
    // Roles → Role entities with permission sets
    // Projects → Project entities in Workspace
    // Environments → Environment entities with tier attribute
    // Instances → Instance entities in Environment
    // Databases → Database entities in Project+Instance with classification
}
```

**Sync strategy**: Full sync on startup, incremental sync via Bus events (same as OPA data loader pattern).

---

## 5. Cedar Policy Templates

### Time-Based Access Control
```cedar
permit(
    principal in Bytebase::Role::"projectDeveloper",
    action == Bytebase::Action::"query",
    resource
) when {
    context.hour >= 9 && context.hour <= 18
    && context.day_of_week != "Saturday"
    && context.day_of_week != "Sunday"
};
```

### Production Migration Governance
```cedar
forbid(
    principal,
    action == Bytebase::Action::"migrate",
    resource in Bytebase::Environment::"production"
) unless {
    context.approval_status == "APPROVED"
    && context.approver_count >= 2
};
```

### PII Data Access
```cedar
forbid(
    principal,
    action == Bytebase::Action::"query",
    resource
) when {
    resource.classification == "PII"
    && !(principal in Bytebase::Role::"workspaceAdmin")
} unless {
    context has "masking_applied" && context.masking_applied == true
};
```

---

## 6. Go Dependencies

```go
require (
    github.com/cedar-policy/cedar-go v0.x.x
)
```

**Binary size**: Cedar Go SDK is lightweight (~3MB), significantly smaller than OPA.

---

## 7. Security & Performance

| Aspect | Detail |
|---|---|
| Schema validation | All policies validated against Cedar schema before loading |
| Entity isolation | Entity store scoped per workspace |
| Conflict analysis | Runs async, results cached, does not block evaluation |
| Evaluation latency | < 0.5ms p99 (Cedar is optimized for authz) |
| Memory footprint | Cedar policy sets are compact, ~1KB per policy |
