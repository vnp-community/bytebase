# Solution: OPA/Rego Policy Engine Plugin

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-002 |
| **CR Reference** | CR-POL-002 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-001 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| `OPAEmbeddedEngine` | **L7 — Plugin** | Implements `PolicyEngine` interface, registered via `init()` |
| `OPASidecarEngine` | **L7 — Plugin** | HTTP client to external OPA server |
| `OPADataLoader` | **L5 — Component** | Syncs Bytebase store data → OPA in-memory store |
| OPA Bundle Manager | **L7 — Plugin** | External bundle loading |
| Rego Templates | **L7 — Plugin** | Pre-built policy files |

---

## 2. Package Structure

```
backend/component/policy/opa/
├── init.go            ← RegisterEngineFactory("opa-embedded", ...) + ("opa-sidecar", ...)
├── embedded.go        ← OPAEmbeddedEngine: in-process OPA Go SDK evaluation
├── sidecar.go         ← OPASidecarEngine: REST API client to OPA server
├── config.go          ← OPAConfig, OPASidecarConfig structs
├── input.go           ← EvaluationRequest → OPA input document conversion
├── data_loader.go     ← OPADataLoader: Bytebase store → OPA store sync
├── bundle.go          ← OPA bundle loading (file, HTTP, S3, git)
├── metrics.go         ← OPA-specific Prometheus metrics
└── templates/
    ├── access_control.rego      ← Time-based access control
    ├── migration_governance.rego ← Schema change governance
    └── dynamic_masking.rego      ← Dynamic data masking
```

---

## 3. Key Design Decisions

### 3.1 Engine Registration — Follows DB Driver Pattern

```go
// backend/component/policy/opa/init.go
func init() {
    policy.RegisterEngineFactory("opa-embedded", func(config json.RawMessage) (policy.PolicyEngine, error) {
        var cfg OPAConfig
        json.Unmarshal(config, &cfg)
        return NewOPAEmbeddedEngine(&cfg)
    })
    policy.RegisterEngineFactory("opa-sidecar", func(config json.RawMessage) (policy.PolicyEngine, error) {
        var cfg OPASidecarConfig
        json.Unmarshal(config, &cfg)
        return NewOPASidecarEngine(&cfg)
    })
}
```

### 3.2 Embedded Engine — In-Process OPA

```go
type OPAEmbeddedEngine struct {
    store       opaStorage.Store     // In-memory OPA data store
    compiler    *ast.Compiler        // Compiled Rego modules
    dataLoader  *OPADataLoader
    mu          sync.RWMutex
}

func (e *OPAEmbeddedEngine) Evaluate(ctx, req) (*PolicyDecision, error) {
    input := e.buildInput(req)  // Convert to OPA input document
    rs, err := rego.New(
        rego.Query(req.QueryPath),
        rego.Input(input),
        rego.Store(e.store),
        rego.Compiler(e.compiler),
    ).Eval(ctx)
    return e.buildDecision(rs)
}
```

### 3.3 Sidecar Engine — REST API Client

```go
type OPASidecarEngine struct {
    client      *http.Client
    baseURL     string              // e.g., "http://localhost:8181"
    retryConfig *RetryConfig        // 3 retries, exponential backoff
}

func (e *OPASidecarEngine) Evaluate(ctx, req) (*PolicyDecision, error) {
    // POST /v1/data/{queryPath}
    // Body: {"input": {...}}
    resp, err := e.client.Post(
        fmt.Sprintf("%s/v1/data/%s", e.baseURL, req.QueryPath),
        "application/json",
        bytes.NewReader(mustJSON(map[string]any{"input": e.buildInput(req)})),
    )
    return e.parseResponse(resp)
}
```

### 3.4 OPA Input Document — Bytebase-Specific Model

Maps `EvaluationRequest` to standardized OPA input:

```json
{
  "input": {
    "subject": {
      "type": "user", "email": "dev@example.com",
      "roles": ["roles/projectDeveloper"],
      "groups": ["groups/dev-team"],
      "attributes": {"department": "engineering"}
    },
    "resource": {
      "type": "database", "name": "databases/production-pg",
      "project": "projects/my-project",
      "environment": "environments/production",
      "engine": "POSTGRES",
      "properties": {"classification": "PII"}
    },
    "action": "bb.databases.query",
    "context": {"hour": 10, "day_of_week": "Saturday", "sql_statement_type": "SELECT"}
  }
}
```

### 3.5 Data Loader — Store → OPA Sync

```go
type OPADataLoader struct {
    store        *store.Store
    opaStore     opaStorage.Store
    syncInterval time.Duration    // Default: 30s
}

func (l *OPADataLoader) SyncAll(ctx) error {
    // Load into data.bytebase namespace:
    // - environments (tier, change_window)
    // - projects (members, settings)
    // - iam/roles (permissions)
    // - iam/groups (memberships)
    // - policies/masking (masking rules)
    // - databases/classification
}

func (l *OPADataLoader) Watch(ctx, bus *bus.Bus) {
    // Listen for Bus events → trigger incremental sync
    // e.g., policy.masking.updated → re-sync masking data
}
```

---

## 4. Go Dependencies

```go
require (
    github.com/open-policy-agent/opa v1.x.x   // OPA Go SDK
)
```

**Binary size impact**: OPA Go SDK adds ~15MB to binary. Acceptable given existing ANTLR4 parsers (TDD §14).

---

## 5. Rego Templates

### Time-Based Access Control

```rego
package bytebase.access.time_restriction
import rego.v1
default allow := false
allow if {
    env := data.bytebase.environments[input.resource.environment]
    input.context.hour >= env.change_window.start_hour
    input.context.hour < env.change_window.end_hour
}
```

### Schema Migration Governance

```rego
package bytebase.governance.migration
import rego.v1
default allow := false
allow if { input.resource.properties.tier != "production" }
allow if {
    input.resource.properties.tier == "production"
    input.context.approval_status == "APPROVED"
    count(input.context.approvers) >= 2
}
```

### Dynamic Data Masking

```rego
package bytebase.masking.dynamic
import rego.v1
masking_level := "NONE" if { "roles/workspaceAdmin" in input.subject.roles }
masking_level := "PARTIAL" if {
    input.resource.properties.classification == "PII"
    "roles/projectDeveloper" in input.subject.roles
}
masking_level := "FULL" if {
    input.resource.properties.classification == "PII"
    not "roles/projectDeveloper" in input.subject.roles
    not "roles/workspaceAdmin" in input.subject.roles
}
```

---

## 6. Security Mitigations

| Concern | Solution |
|---|---|
| Rego execution sandbox | OPA embedded with restricted builtins (no `http.send`, no `opa.runtime`) |
| Rego policy injection | Mandatory `ast.Compile()` validation before `LoadPolicy()` |
| OPA data isolation | Data loader scoped per workspace, no cross-workspace data in OPA store |
| Sidecar communication | Localhost-only or mTLS, configurable in `OPASidecarConfig` |
| Bundle signatures | Cryptographic verification for remote bundles |

---

## 7. Performance Targets

| Metric | Target |
|---|---|
| Embedded single evaluation | < 1ms (p99) |
| Sidecar single evaluation | < 10ms (p99) |
| Batch 100 evaluations | < 50ms |
| Data sync full | < 2s |
| Data sync incremental | < 100ms |
