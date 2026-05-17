# Change Request: OPA/Rego Policy Engine Plugin

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-POL-002                                               |
| **Title**          | OPA/Rego Policy Engine Plugin                            |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-POL-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **OPA (Open Policy Agent) / Rego Policy Engine Plugin** — implement `PolicyEngine` interface (CR-POL-001) với 3 deployment modes: embedded (OPA Go library), sidecar (OPA server), và OPAL-managed (distributed fleet). OPA là policy engine phổ biến nhất (CNCF Graduated), hỗ trợ Rego language cho các chính sách phức tạp.

### 1.2 Bối cảnh
OPA được sử dụng rộng rãi trong enterprise cho:
- **API authorization** — quyết định ai được gọi API nào
- **Data filtering** — lọc dữ liệu dựa trên chính sách
- **Infrastructure governance** — kiểm soát Terraform, Kubernetes resources
- **Compliance** — đảm bảo tuân thủ quy định

Trong ngữ cảnh Bytebase, OPA có thể bổ sung cho hệ thống IAM hiện tại bằng:
- **Fine-grained database access policies** — vượt xa RBAC (ví dụ: "DBA chỉ được modify schema ngoài giờ peak")
- **Dynamic masking rules** — masking level dựa trên context phức tạp (time, IP, query pattern)
- **Schema change governance** — policies cho phép/từ chối migration dựa trên SQL analysis
- **Cross-system policy reuse** — cùng Rego rules cho Bytebase, K8s, API gateway

### 1.3 Mục tiêu
- 3 deployment modes cho OPA (embedded, sidecar, OPAL-managed)
- Rego policy loading, compilation, and evaluation
- Bytebase-specific OPA data model (input/data documents)
- Rego policy templates cho common Bytebase use cases
- Integration với OPA bundle mechanism
- Prometheus metrics cho OPA evaluation

---

## 2. Yêu cầu chức năng

### FR-001: OPA Embedded Engine

OPA Go library chạy in-process — lowest latency, simplest deployment:

```go
// OPAEmbeddedEngine implements PolicyEngine using the OPA Go SDK.
type OPAEmbeddedEngine struct {
    rego        *rego.Rego
    store       opaStorage.Store    // In-memory OPA store
    compiler    *ast.Compiler       // Compiled Rego modules
    bundles     map[string]*bundle.Bundle
    dataLoader  *OPADataLoader      // Loads Bytebase data into OPA store
    metrics     *OPAMetrics
    mu          sync.RWMutex
}

func NewOPAEmbeddedEngine(config *OPAConfig) (*OPAEmbeddedEngine, error) {
    // Initialize OPA with in-memory store
    // Compile base Rego modules
    // Start data sync goroutine
}

func (e *OPAEmbeddedEngine) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error) {
    // 1. Convert EvaluationRequest → OPA input document
    input := e.buildInput(req)

    // 2. Evaluate Rego query
    rs, err := rego.New(
        rego.Query(req.QueryPath),
        rego.Input(input),
        rego.Store(e.store),
        rego.Compiler(e.compiler),
    ).Eval(ctx)

    // 3. Convert OPA result → PolicyDecision
    return e.buildDecision(rs)
}
```

### FR-002: OPA Sidecar Engine

OPA server chạy như sidecar — tách biệt, hot-reloadable:

```go
// OPASidecarEngine implements PolicyEngine using OPA REST API.
type OPASidecarEngine struct {
    client      *http.Client
    baseURL     string           // e.g., "http://localhost:8181"
    apiVersion  string           // "v1"
    retryConfig *RetryConfig
    metrics     *OPAMetrics
}

func (e *OPASidecarEngine) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error) {
    // POST /v1/data/{queryPath}
    // Body: {"input": {...}}
    input := e.buildInput(req)
    resp, err := e.client.Post(
        fmt.Sprintf("%s/v1/data/%s", e.baseURL, req.QueryPath),
        "application/json",
        bytes.NewReader(mustJSON(map[string]interface{}{"input": input})),
    )
    // Parse response → PolicyDecision
}
```

### FR-003: Bytebase OPA Input Document Model

Chuẩn hóa input document cho OPA evaluation trong ngữ cảnh Bytebase:

```json
{
  "input": {
    "subject": {
      "type": "user",
      "email": "developer@example.com",
      "roles": ["roles/projectDeveloper"],
      "groups": ["groups/dev-team"],
      "attributes": {
        "department": "engineering",
        "ip_address": "10.0.1.50",
        "auth_method": "sso"
      }
    },
    "resource": {
      "type": "database",
      "name": "databases/production-pg",
      "project": "projects/my-project",
      "environment": "environments/production",
      "instance": "instances/pg-primary",
      "engine": "POSTGRES",
      "properties": {
        "classification": "PII",
        "tier": "production"
      }
    },
    "action": "bb.databases.query",
    "context": {
      "timestamp": "2026-05-17T10:30:00Z",
      "day_of_week": "Saturday",
      "hour": 10,
      "request_id": "req-123",
      "sql_statement_type": "SELECT",
      "affected_rows_estimate": 1000
    }
  }
}
```

### FR-004: Bytebase OPA Data Document Model

Data documents cung cấp context cho policy evaluation:

```json
{
  "data": {
    "bytebase": {
      "environments": {
        "production": {
          "tier": "production",
          "approval_required": true,
          "change_window": {"start": "02:00", "end": "06:00", "timezone": "UTC"}
        }
      },
      "policies": {
        "masking_rules": [...],
        "access_grants": [...],
        "data_classification": {...}
      },
      "projects": {
        "my-project": {
          "members": [...],
          "settings": {...}
        }
      }
    }
  }
}
```

### FR-005: OPA Data Loader

Sync Bytebase data vào OPA store:

```go
type OPADataLoader struct {
    store       *store.Store
    opaStore    opaStorage.Store
    syncInterval time.Duration   // Default: 30s
    lastSync    time.Time
}

// SyncAll loads all relevant Bytebase data into OPA store.
func (l *OPADataLoader) SyncAll(ctx context.Context) error {
    // Load environments, projects, policies, roles, groups
    // into data.bytebase namespace
}

// SyncIncremental syncs only changed data since last sync.
func (l *OPADataLoader) SyncIncremental(ctx context.Context) error

// Watch listens for Bus events and triggers incremental sync.
func (l *OPADataLoader) Watch(ctx context.Context, bus *component.Bus)
```

### FR-006: Rego Policy Templates

Pre-built Rego templates cho common Bytebase use cases:

```rego
# Template: Database Access Control — Time-based
package bytebase.access.time_restriction

import rego.v1

default allow := false

allow if {
    input.context.hour >= data.bytebase.environments[input.resource.environment].change_window.start_hour
    input.context.hour < data.bytebase.environments[input.resource.environment].change_window.end_hour
}

deny_reason := "Outside approved change window" if {
    not allow
}
```

```rego
# Template: Schema Migration Governance
package bytebase.governance.migration

import rego.v1

default allow := false

# Allow schema changes only with approval in production
allow if {
    input.resource.properties.tier != "production"
}

allow if {
    input.resource.properties.tier == "production"
    input.context.approval_status == "APPROVED"
    count(input.context.approvers) >= 2
}

# Deny destructive operations on production
deny if {
    input.resource.properties.tier == "production"
    input.context.sql_statement_type in {"DROP", "TRUNCATE"}
    not input.context.has_backup
}
```

```rego
# Template: Dynamic Data Masking
package bytebase.masking.dynamic

import rego.v1

# Determine masking level based on user context
masking_level := "NONE" if {
    "roles/workspaceAdmin" in input.subject.roles
}

masking_level := "NONE" if {
    input.resource.properties.classification != "PII"
}

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

### FR-007: OPA Bundle Support

Support OPA bundles cho policy packaging:

```go
type OPABundleConfig struct {
    // Bundle sources
    Sources []OPABundleSource

    // Polling interval for remote bundles
    PollingInterval time.Duration  // Default: 60s

    // Signature verification
    VerifySignatures bool
    SigningKey       string
}

type OPABundleSource struct {
    Name     string
    Type     string  // "file", "http", "s3", "gcs", "git"
    URL      string
    Auth     string  // Auth method for remote bundles
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| OPAEmbeddedEngine                  | `backend/component/policy/opa/embedded.go`           | New: OPA Go SDK engine implementation        |
| OPASidecarEngine                   | `backend/component/policy/opa/sidecar.go`            | New: OPA REST API engine implementation      |
| OPAConfig                          | `backend/component/policy/opa/config.go`             | New: OPA engine configuration                |
| OPADataLoader                      | `backend/component/policy/opa/data_loader.go`        | New: Bytebase → OPA data sync                |
| OPA Input Builder                  | `backend/component/policy/opa/input.go`              | New: EvaluationRequest → OPA input conversion|
| Rego Templates                     | `backend/component/policy/opa/templates/`            | New: pre-built Rego policy templates         |
| OPA Bundle Manager                 | `backend/component/policy/opa/bundle.go`             | New: bundle loading and management           |
| Proto: OPAEngineConfig             | `proto/store/policy_engine.proto`                    | Add: OPA-specific configuration fields       |
| Plugin registration                | `backend/component/policy/opa/init.go`               | New: engine registration via init()          |

### 3.1 Go Dependencies

```go
// go.mod additions
require (
    github.com/open-policy-agent/opa v1.x.x       // OPA Go SDK
    github.com/open-policy-agent/opa/rego          // Rego evaluation
    github.com/open-policy-agent/opa/ast           // AST compilation
    github.com/open-policy-agent/opa/storage       // Storage interface
    github.com/open-policy-agent/opa/bundle        // Bundle support
)
```

### 3.2 Engine Registration Pattern

Follows the existing Bytebase plugin registration pattern (similar to DB drivers):

```go
// In backend/component/policy/opa/init.go
func init() {
    policy.RegisterEngineFactory("opa-embedded", func(config json.RawMessage) (policy.PolicyEngine, error) {
        var cfg OPAConfig
        if err := json.Unmarshal(config, &cfg); err != nil {
            return nil, err
        }
        return NewOPAEmbeddedEngine(&cfg)
    })

    policy.RegisterEngineFactory("opa-sidecar", func(config json.RawMessage) (policy.PolicyEngine, error) {
        var cfg OPASidecarConfig
        if err := json.Unmarshal(config, &cfg); err != nil {
            return nil, err
        }
        return NewOPASidecarEngine(&cfg)
    })
}
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Rego execution sandbox           | OPA embedded runs in-process with restricted builtins         |
| Network policy for sidecar       | OPA sidecar communicates only via localhost or mTLS           |
| Rego policy injection            | All Rego must pass `ast.Compile()` validation before loading  |
| OPA data exposure                | Data loader only syncs workspace-scoped data, no cross-workspace |
| Bundle signature                 | Remote bundles can require cryptographic signature verification |
| OPA decision logging             | Decision logs can be routed to audit_log table                |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | OPA embedded: load and evaluate simple Rego           | Correct allow/deny result                  |
| TC-002  | OPA embedded: compile invalid Rego                    | Compilation error returned                 |
| TC-003  | OPA embedded: evaluate with data documents            | Data accessible in Rego via `data.*`       |
| TC-004  | OPA sidecar: evaluate via REST API                    | HTTP 200 with decision                     |
| TC-005  | OPA sidecar: connection failure                       | Error with retry + fallback to CEL         |
| TC-006  | OPA data loader: sync environments                    | Environments available in `data.bytebase`  |
| TC-007  | OPA data loader: incremental sync on policy change    | Updated policy reflected in OPA store      |
| TC-008  | Template: time-based access control                   | Access denied outside window, allowed inside |
| TC-009  | Template: migration governance (production 2-approver)| Denied without 2 approvers                 |
| TC-010  | Template: dynamic masking (PII classification)        | Correct masking level per role             |
| TC-011  | OPA bundle: load from file                            | Policies available for evaluation          |
| TC-012  | Concurrent evaluation: 1000 parallel requests         | All complete < 100ms, no race conditions   |
| TC-013  | EvaluateBatch: 50 requests in single call             | All results returned correctly             |

---

## 6. Rollout Plan

| Phase   | Mô tả                                         | Timeline       |
|---------|------------------------------------------------|----------------|
| Phase 1 | OPA Go SDK integration + embedded engine       | Sprint 1-2     |
| Phase 2 | Input/data document model + data loader        | Sprint 2       |
| Phase 3 | Sidecar engine + REST API client               | Sprint 3       |
| Phase 4 | Rego templates (access, masking, governance)   | Sprint 3       |
| Phase 5 | Bundle support + file/HTTP loading             | Sprint 4       |
| Phase 6 | Integration testing + performance benchmarks   | Sprint 4-5     |
