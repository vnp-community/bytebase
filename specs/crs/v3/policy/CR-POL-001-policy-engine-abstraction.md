# Change Request: Policy Engine Abstraction Layer

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-POL-001                                               |
| **Title**          | Policy Engine Abstraction Layer                          |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P0 — Critical                                            |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế và triển khai **Policy Engine Abstraction Layer** — một interface chuẩn hóa cho tất cả các policy engine (OPA, Cedar, CEL, Kyverno...), cho phép Bytebase đánh giá chính sách từ nhiều engine khác nhau thông qua một unified API. Layer này là nền tảng cho toàn bộ hệ thống multi-engine policy management.

### 1.2 Bối cảnh
Hiện tại Bytebase quản lý chính sách qua 2 cơ chế riêng biệt:

1. **OrgPolicyService** (`backend/api/v1/org_policy_service.go`, 35KB) — CRUD policy lưu JSONB trong `policy` table. Hỗ trợ các policy types: `MASKING`, `MASKING_EXCEPTION`, `ROLLOUT_POLICY`, `SLOW_QUERY`, `DATA_SOURCE_QUERY`, `TAG`, `DISABLE_COPY_DATA`.

2. **IAM Manager** (`backend/component/iam/`) — CEL-based permission evaluation cho RBAC, group membership, workspace/project-level authorization.

**Vấn đề**:
- Policy types **hardcoded** trong protobuf enum → không extensible cho external policy formats (Rego, Cedar DSL).
- Policy evaluation **inline** trong service code (MaskingEvaluator, ACL interceptor) → không pluggable.
- Không có abstraction cho **external policy engines** — mỗi engine evaluation phải implement riêng.
- Không hỗ trợ **multiple policy languages** — chỉ CEL và hardcoded Go logic.
- Không có **policy decision caching** — mỗi request evaluate lại toàn bộ policy chain.
- Thiếu **policy metadata** — không track policy source (git, API, UI), version, hay author.

### 1.3 Mục tiêu
- Unified `PolicyEngine` interface cho tất cả policy evaluation engines
- `PolicyManager` orchestrator quản lý multiple engines đồng thời
- Engine lifecycle management (init, health, reload, close)
- Policy decision caching với configurable TTL
- Backward compatible với existing OrgPolicyService và IAM Manager
- Foundation cho tất cả CR-POL-* khác

---

## 2. Yêu cầu chức năng

### FR-001: PolicyEngine Interface

Định nghĩa Go interface chuẩn cho policy engines:

```go
// PolicyEngine defines the contract for policy evaluation backends.
type PolicyEngine interface {
    // Name returns the engine identifier (e.g., "opa", "cedar", "cel", "kyverno").
    Name() string

    // Type returns the engine type classification.
    Type() EngineType

    // Evaluate evaluates a policy decision for the given input.
    // Returns a PolicyDecision with allow/deny result and detailed reasoning.
    Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error)

    // EvaluateBatch evaluates multiple policy decisions in a single call.
    // Optimized for engines that support batch evaluation (e.g., OPA).
    EvaluateBatch(ctx context.Context, reqs []*EvaluationRequest) ([]*PolicyDecision, error)

    // LoadPolicy loads a policy definition into the engine.
    // For embedded engines (OPA), this compiles the policy.
    // For remote engines (OPAL-managed OPA), this is a no-op.
    LoadPolicy(ctx context.Context, policy *PolicyDefinition) error

    // UnloadPolicy removes a policy from the engine.
    UnloadPolicy(ctx context.Context, policyID string) error

    // ListPolicies returns all loaded policies in the engine.
    ListPolicies(ctx context.Context) ([]*PolicyInfo, error)

    // ValidatePolicy checks if a policy definition is syntactically valid.
    ValidatePolicy(ctx context.Context, policy *PolicyDefinition) (*ValidationResult, error)

    // Healthy performs a health check on the engine.
    Healthy(ctx context.Context) error

    // Close cleans up engine resources.
    Close() error
}

// EngineType classifies the policy engine.
type EngineType int
const (
    EngineTypeEmbedded EngineType = iota  // In-process (OPA Go library, CEL)
    EngineTypeSidecar                      // Local sidecar (OPA server, Cedar agent)
    EngineTypeRemote                       // Remote service (OPAL-managed OPA fleet)
)
```

### FR-002: EvaluationRequest & PolicyDecision

```go
// EvaluationRequest represents a policy evaluation request.
type EvaluationRequest struct {
    // PolicyID identifies which policy to evaluate (optional — engine may evaluate all).
    PolicyID    string

    // Resource describes what is being accessed/modified.
    Resource    *PolicyResource

    // Subject describes who is performing the action.
    Subject     *PolicySubject

    // Action describes what operation is being performed.
    Action      string

    // Context provides additional evaluation context.
    Context     map[string]interface{}

    // Engine-specific query path (e.g., "data.bytebase.authz.allow" for OPA).
    QueryPath   string
}

// PolicyResource represents the target resource.
type PolicyResource struct {
    Type        string            // "database", "instance", "project", "table", "column"
    ID          string            // Resource identifier
    Properties  map[string]string // Engine-specific attributes
    Parent      *PolicyResource   // Hierarchical parent (e.g., table → database → instance)
}

// PolicySubject represents the requesting principal.
type PolicySubject struct {
    Type        string            // "user", "service_account", "workload_identity"
    ID          string            // Principal identifier
    Groups      []string          // Group memberships
    Roles       []string          // Assigned roles
    Attributes  map[string]string // Additional attributes (department, team, etc.)
}

// PolicyDecision represents the result of a policy evaluation.
type PolicyDecision struct {
    // Allowed indicates whether the action is permitted.
    Allowed     bool

    // Reason provides human-readable explanation.
    Reason      string

    // Details contains engine-specific decision details.
    Details     map[string]interface{}

    // Obligations are actions that must be performed (e.g., "apply masking level PARTIAL").
    Obligations []*PolicyObligation

    // EvaluationTime is how long the evaluation took.
    EvaluationTime time.Duration

    // Engine identifies which engine made the decision.
    Engine      string

    // PolicyID identifies which policy produced this decision.
    PolicyID    string
}

// PolicyObligation represents a required action resulting from policy evaluation.
type PolicyObligation struct {
    Type        string            // "mask", "audit", "notify", "rate_limit"
    Parameters  map[string]string // Obligation-specific parameters
}
```

### FR-003: PolicyDefinition — Universal Policy Container

```go
// PolicyDefinition is the storage-agnostic representation of a policy.
type PolicyDefinition struct {
    // Identity
    ID          string
    Name        string
    Description string
    Version     int

    // Classification
    Category    PolicyCategory     // ACCESS, MASKING, GOVERNANCE, COMPLIANCE, CUSTOM
    Scope       PolicyScope        // WORKSPACE, ENVIRONMENT, PROJECT, DATABASE, INSTANCE

    // Content
    Language    PolicyLanguage     // REGO, CEDAR, CEL, YAML, JSON
    Source      string             // Raw policy source code
    Compiled    []byte             // Engine-specific compiled form (optional)

    // Metadata
    EngineID    string             // Target engine for evaluation
    Tags        map[string]string  // Arbitrary tags for filtering
    Labels      []string           // Labels for grouping

    // Lifecycle
    Status      PolicyStatus       // DRAFT, TESTING, STAGING, ACTIVE, DEPRECATED, ARCHIVED
    Author      string             // Creator principal
    GitRef      string             // Git commit/branch reference (if GitOps-managed)
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type PolicyLanguage int
const (
    PolicyLanguageRego   PolicyLanguage = iota // OPA Rego
    PolicyLanguageCedar                        // Amazon Cedar DSL
    PolicyLanguageCEL                          // Google Common Expression Language
    PolicyLanguageYAML                         // Kyverno-style YAML policies
    PolicyLanguageJSON                         // JSON-based policy definitions
)

type PolicyCategory int
const (
    PolicyCategoryAccess      PolicyCategory = iota // Who can access what
    PolicyCategoryMasking                           // Data masking rules
    PolicyCategoryGovernance                        // Schema change governance
    PolicyCategoryCompliance                        // Compliance/regulatory
    PolicyCategoryCustom                            // User-defined
)

type PolicyScope int
const (
    PolicyScopeWorkspace   PolicyScope = iota
    PolicyScopeEnvironment
    PolicyScopeProject
    PolicyScopeDatabase
    PolicyScopeInstance
)
```

### FR-004: PolicyManager Orchestrator

```go
// PolicyManager orchestrates multiple policy engines and handles evaluation routing.
type PolicyManager struct {
    engines     map[string]PolicyEngine       // engine_id → engine
    store       *store.Store                  // Policy persistence
    cache       *PolicyDecisionCache          // Decision cache
    config      *PolicyManagerConfig          // Runtime config
    metrics     *PolicyMetrics                // Prometheus metrics
    bus         *component.Bus               // Event bus for policy change notifications
    fallback    PolicyEngine                  // Fallback engine (CEL/internal) when primary unavailable
}

// Evaluate routes to the appropriate engine and evaluates.
func (m *PolicyManager) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error)

// EvaluateChain evaluates across multiple engines with configurable strategy.
// Strategy: ALL_MUST_ALLOW, ANY_ALLOW, FIRST_MATCH, PRIORITY_ORDER
func (m *PolicyManager) EvaluateChain(ctx context.Context, req *EvaluationRequest, strategy EvalStrategy) (*PolicyDecision, error)

// RegisterEngine adds a policy engine to the manager.
func (m *PolicyManager) RegisterEngine(engine PolicyEngine) error

// UnregisterEngine removes a policy engine.
func (m *PolicyManager) UnregisterEngine(engineID string) error

// ReloadPolicies reloads all policies from store into engines.
func (m *PolicyManager) ReloadPolicies(ctx context.Context) error

// HealthCheck checks all registered engines.
func (m *PolicyManager) HealthCheck(ctx context.Context) map[string]error
```

### FR-005: Multi-Engine Evaluation Strategy

```go
type EvalStrategy int
const (
    // EvalStrategyAllMustAllow — all engines must allow (most restrictive).
    EvalStrategyAllMustAllow EvalStrategy = iota

    // EvalStrategyAnyAllow — any engine allowing is sufficient.
    EvalStrategyAnyAllow

    // EvalStrategyFirstMatch — first engine with a definitive answer wins.
    EvalStrategyFirstMatch

    // EvalStrategyPriorityOrder — evaluate in priority order, first deny wins.
    EvalStrategyPriorityOrder

    // EvalStrategyMergeObligations — allow if all allow, merge all obligations.
    EvalStrategyMergeObligations
)
```

### FR-006: Policy Decision Caching

- LRU cache với configurable TTL (default: 30 giây cho access policies, 5 phút cho masking policies)
- Cache key: `hash(engine_id + policy_id + subject + resource + action)`
- Cache invalidation khi policy updated hoặc reloaded
- Cache bypass option cho audit/compliance evaluation
- Metrics: hit/miss ratio, eviction count, cache latency

### FR-007: CEL Engine Adapter (Backward Compatibility)

Wrap existing IAM Manager và OrgPolicyService evaluation logic thành `PolicyEngine` implementation:

```go
// CELEngine wraps the existing CEL-based IAM and OrgPolicy evaluation.
type CELEngine struct {
    iamManager     *iam.Manager
    store          *store.Store
    licenseService enterprise.LicenseService
}

func (e *CELEngine) Name() string { return "cel-internal" }
func (e *CELEngine) Type() EngineType { return EngineTypeEmbedded }
func (e *CELEngine) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error) {
    // Route to existing IAM Manager or OrgPolicy evaluation based on PolicyCategory
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| PolicyEngine interface             | `backend/component/policy/engine.go`                 | New: engine interface + types                |
| PolicyManager                      | `backend/component/policy/manager.go`                | New: multi-engine orchestrator               |
| PolicyDefinition types             | `backend/component/policy/definition.go`             | New: universal policy container              |
| EvaluationRequest/Decision         | `backend/component/policy/evaluation.go`             | New: evaluation request/response types       |
| PolicyDecisionCache                | `backend/component/policy/cache.go`                  | New: LRU cache with TTL for decisions        |
| CELEngine adapter                  | `backend/component/policy/cel_engine.go`             | New: wrap existing IAM/OrgPolicy as engine   |
| PolicyMetrics                      | `backend/component/policy/metrics.go`                | New: Prometheus metrics for policy ops       |
| Proto: PolicyEngineConfig          | `proto/store/setting.proto`                          | Add: workspace-level engine configuration    |
| Proto: PolicyDefinition            | `proto/store/policy_engine.proto`                    | New: policy definition protobuf              |
| Store: policy_engine table         | `backend/store/policy_engine.go`                     | New: policy engine registration persistence  |
| Store: policy_definition table     | `backend/store/policy_definition.go`                 | New: external policy definitions persistence |
| Server bootstrap                   | `backend/server/server.go`                           | Add: PolicyManager initialization at step 5.5|
| IAM Manager integration           | `backend/component/iam/manager.go`                   | Refactor: delegate to PolicyManager          |
| ACL Interceptor integration       | `backend/api/v1/acl.go`                              | Refactor: use PolicyManager.Evaluate()       |

### 3.1 Proto Schema Changes

```protobuf
// New file: proto/store/policy_engine.proto
syntax = "proto3";
package bytebase.store;

message PolicyEngineConfig {
  enum EngineType {
    ENGINE_UNSPECIFIED = 0;
    CEL_INTERNAL = 1;        // Built-in CEL (backward compatible)
    OPA_EMBEDDED = 2;        // Embedded OPA Go library
    OPA_SIDECAR = 3;         // OPA server (sidecar/remote)
    OPAL_MANAGED = 4;        // OPAL-managed OPA fleet
    CEDAR_EMBEDDED = 5;      // Embedded Cedar engine
    CEDAR_REMOTE = 6;        // Remote Cedar service
    KYVERNO = 7;             // Kyverno (K8s-native, remote)
    OPENFGA = 8;             // OpenFGA (ReBAC, remote)
    CUSTOM_HTTP = 9;         // Custom HTTP-based engine
  }

  string engine_id = 1;
  EngineType engine_type = 2;
  string display_name = 3;
  bool enabled = 4;
  int32 priority = 5;           // Lower = higher priority in chain evaluation

  // Engine-specific connection config (JSON)
  string connection_config = 6;

  // Evaluation strategy when multiple engines registered
  EvaluationStrategy strategy = 7;

  // Health check interval (seconds)
  int32 health_check_interval = 8;
}

enum EvaluationStrategy {
  STRATEGY_UNSPECIFIED = 0;
  ALL_MUST_ALLOW = 1;
  ANY_ALLOW = 2;
  FIRST_MATCH = 3;
  PRIORITY_ORDER = 4;
  MERGE_OBLIGATIONS = 5;
}

// PolicyDefinition stored in policy_definition table
message StoredPolicyDefinition {
  string id = 1;
  string name = 2;
  string description = 3;
  int32 version = 4;

  PolicyCategory category = 5;
  PolicyScope scope = 6;
  PolicyLanguage language = 7;

  string source = 8;           // Raw policy source code
  bytes compiled = 9;          // Compiled form

  string engine_id = 10;       // Target engine
  map<string, string> tags = 11;

  PolicyStatus status = 12;
  string author = 13;
  string git_ref = 14;
}

enum PolicyCategory {
  CATEGORY_UNSPECIFIED = 0;
  ACCESS = 1;
  MASKING = 2;
  GOVERNANCE = 3;
  COMPLIANCE = 4;
  CUSTOM = 5;
}

enum PolicyScope {
  SCOPE_UNSPECIFIED = 0;
  WORKSPACE = 1;
  ENVIRONMENT = 2;
  PROJECT = 3;
  DATABASE = 4;
  INSTANCE = 5;
}

enum PolicyLanguage {
  LANGUAGE_UNSPECIFIED = 0;
  REGO = 1;
  CEDAR = 2;
  CEL = 3;
  YAML = 4;
  JSON = 5;
}

enum PolicyStatus {
  STATUS_UNSPECIFIED = 0;
  DRAFT = 1;
  TESTING = 2;
  STAGING = 3;
  ACTIVE = 4;
  DEPRECATED = 5;
  ARCHIVED = 6;
}
```

### 3.2 Database Schema Changes

```sql
-- New table: policy_engine — stores registered engine configurations
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
    last_health_check TIMESTAMPTZ,
    health_status TEXT NOT NULL DEFAULT 'UNKNOWN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- New table: policy_definition — stores external policy definitions
CREATE TABLE policy_definition (
    id TEXT NOT NULL,
    workspace TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    category TEXT NOT NULL,
    scope TEXT NOT NULL,
    scope_resource TEXT NOT NULL DEFAULT '',  -- scope-specific resource ID
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

CREATE INDEX idx_policy_definition_engine ON policy_definition(engine_id);
CREATE INDEX idx_policy_definition_category ON policy_definition(workspace, category, status);
CREATE INDEX idx_policy_definition_scope ON policy_definition(workspace, scope, scope_resource);
```

### 3.3 Bootstrap Integration

```
Server Bootstrap (updated):
  └─ NewServer(ctx, profile)
       ├─ 1.  StartMetadataInstance()
       ├─ 2.  store.New(pgURL)
       ├─ 2.5 secretManager = secret.NewManager(store, vaultConfig)
       ├─ 3.  migrator.MigrateSchema()
       ├─ 4.  enterprise.NewLicenseService()
       ├─ 5.  iam.NewManager()
       ├─ 5.5 policyManager = policy.NewManager(store, iamManager, licenseService, bus) ← NEW
       │       ├─ Register CELEngine (always, backward compatible)
       │       ├─ Register configured external engines (Enterprise only)
       │       └─ Load active PolicyDefinitions from store into engines
       ├─ 6.  webhook.NewManager()
       ├─ 7.  dbfactory.New()
       ...
```

### 3.4 Migration Path cho Existing Code

**Phase 1: Adapter Pattern**

Wrap existing evaluation logic — zero breaking changes:

```go
// ACL Interceptor (current):
allowed, err := iamManager.CheckPermission(ctx, permission, user, workspaceID, projectIDs...)

// ACL Interceptor (after CR-POL-001):
req := &EvaluationRequest{
    Resource: &PolicyResource{Type: "project", ID: projectID},
    Subject:  &PolicySubject{Type: user.Type, ID: user.Email, Roles: roles, Groups: groups},
    Action:   permission,
}
decision, err := policyManager.Evaluate(ctx, req)
// CELEngine internally delegates to iamManager.CheckPermission()
```

**Phase 2: External Engine Integration**

After CR-POL-002/003 are implemented, external engines participate in evaluation chain:

```go
// PolicyManager routes to registered engines based on strategy
decision, err := policyManager.EvaluateChain(ctx, req, EvalStrategyPriorityOrder)
// → CELEngine.Evaluate() (priority=0, always first)
// → OPAEngine.Evaluate() (priority=10, if registered)
// → CedarEngine.Evaluate() (priority=20, if registered)
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Engine credentials at rest       | Engine connection config encrypted via SecretManager (CR-VLT-001) |
| Policy source code exposure      | PolicyDefinition.source requires WORKSPACE_ADMIN to read      |
| Cache poisoning                  | Cache keys include workspace scope, TTL prevents stale decisions |
| Engine unavailability            | Configurable fallback to CEL internal engine with alert       |
| Malicious policy injection       | ValidatePolicy() mandatory before LoadPolicy()                |
| Cross-workspace isolation        | All queries scoped by workspace, engine instances isolated    |
| Decision logging sensitivity     | Subject attributes in decision logs are redactable            |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Initialize PolicyManager with CELEngine only          | Backward compatible, all existing tests pass |
| TC-002  | Register/unregister external engine                   | Engine lifecycle managed correctly         |
| TC-003  | Evaluate with single engine                           | Correct decision returned                  |
| TC-004  | EvaluateChain with ALL_MUST_ALLOW (all allow)         | Allowed=true                              |
| TC-005  | EvaluateChain with ALL_MUST_ALLOW (one deny)          | Allowed=false, deny reason from denying engine |
| TC-006  | EvaluateChain with PRIORITY_ORDER                     | Higher priority engine decision wins       |
| TC-007  | EvaluateChain with MERGE_OBLIGATIONS                  | Allowed + merged obligations from all engines |
| TC-008  | Policy decision cache hit                              | Return cached, no engine call              |
| TC-009  | Policy decision cache invalidation on policy update   | Fresh evaluation on next request           |
| TC-010  | Engine health check failure + fallback                | CEL engine handles evaluation              |
| TC-011  | LoadPolicy with invalid Rego syntax                   | ValidationResult with errors returned      |
| TC-012  | Concurrent evaluation across multiple engines         | Thread-safe, no race conditions            |
| TC-013  | Metrics: policy_evaluation_total counter              | Incremented per evaluation, labeled by engine |
| TC-014  | EvaluateBatch with 100 requests                       | All results returned, performance < 50ms   |

---

## 6. Rollout Plan

| Phase   | Mô tả                                         | Timeline       |
|---------|------------------------------------------------|----------------|
| Phase 1 | Interface definition + types                   | Sprint 1       |
| Phase 2 | PolicyManager + CELEngine adapter              | Sprint 1-2     |
| Phase 3 | Policy decision cache + metrics                | Sprint 2       |
| Phase 4 | Proto changes + DB migration + store methods   | Sprint 2-3     |
| Phase 5 | ACL/MaskingEvaluator refactor to use PolicyManager | Sprint 3   |
| Phase 6 | Integration testing + backward compatibility   | Sprint 3-4     |
| Phase 7 | Setting UI for engine configuration            | Sprint 4       |
