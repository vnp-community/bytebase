# Change Request: Cedar Policy Engine Plugin

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-POL-003                                               |
| **Title**          | Cedar Policy Engine Plugin                               |
| **Plan**           | ENTERPRISE                                               |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-17                                               |
| **Author**         | VNP AI Ops Team                                          |
| **Dependencies**   | CR-POL-001                                               |

---

## 1. Tổng quan

### 1.1 Mô tả
Triển khai **Cedar Policy Engine Plugin** — implement `PolicyEngine` interface (CR-POL-001) sử dụng Amazon Cedar language. Cedar cung cấp formal verification capabilities, thiết kế cho authorization use cases với cú pháp dễ đọc hơn Rego và khả năng phân tích chính sách tĩnh (static analysis).

### 1.2 Bối cảnh
Cedar (developed by Amazon, open-sourced) là policy language thế hệ mới, thiết kế đặc biệt cho:
- **Authorization decisions** — permit/forbid với conditions
- **Attribute-Based Access Control (ABAC)** — policies dựa trên attributes của principal/resource
- **Formal verification** — có thể chứng minh mathematically rằng policies không conflict
- **IAM-style policies** — cú pháp giống AWS IAM policies, dễ hiểu cho security teams

Trong ngữ cảnh Bytebase, Cedar phù hợp cho:
- **Database access authorization** — "Permit developer to query production if department == 'engineering'"
- **Schema change approval** — "Forbid DROP TABLE unless principal is DBA and time in change_window"
- **Data masking policies** — "When action == 'query' and resource.classification == 'PII', apply masking"
- **Compliance policies** — formal verification đảm bảo no unintended access

### 1.3 Mục tiêu
- Cedar engine implementation (Go bindings via cedar-go)
- Bytebase-specific Cedar schema (entity types, actions)
- Cedar policy templates cho common use cases
- Policy conflict detection via Cedar's formal analysis
- Integration với PolicyManager (CR-POL-001)

---

## 2. Yêu cầu chức năng

### FR-001: Cedar Engine Implementation

```go
// CedarEngine implements PolicyEngine using the Cedar policy language.
type CedarEngine struct {
    policySet   *cedar.PolicySet      // Compiled Cedar policies
    schema      *cedar.Schema         // Entity/action schema
    entities    *cedar.EntityStore    // Entity data (principals, resources)
    dataLoader  *CedarDataLoader      // Loads Bytebase data into Cedar entities
    metrics     *CedarMetrics
    mu          sync.RWMutex
}

func NewCedarEngine(config *CedarConfig) (*CedarEngine, error) {
    // Initialize Cedar with Bytebase schema
    // Load entity definitions
}

func (e *CedarEngine) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error) {
    // 1. Build Cedar authorization request
    authzReq := cedar.Request{
        Principal: e.buildPrincipal(req.Subject),
        Action:    cedar.EntityUID{Type: "Action", ID: req.Action},
        Resource:  e.buildResource(req.Resource),
        Context:   e.buildContext(req.Context),
    }

    // 2. Evaluate
    decision, diagnostics := e.policySet.IsAuthorized(e.entities, authzReq)

    // 3. Convert to PolicyDecision
    return &PolicyDecision{
        Allowed: decision == cedar.Allow,
        Reason:  diagnostics.Reason(),
        Engine:  "cedar",
    }, nil
}
```

### FR-002: Bytebase Cedar Schema

Định nghĩa Cedar entity types và actions cho Bytebase domain:

```cedar
// Bytebase Cedar Schema
namespace Bytebase {
    // Entity types
    entity User in [Group, Role] {
        email: String,
        department: String,
        auth_method: String,
    };

    entity ServiceAccount in [Role] {
        name: String,
    };

    entity Group {
        name: String,
        description: String,
    };

    entity Role {
        name: String,
        permissions: Set<String>,
    };

    entity Workspace {
        name: String,
        plan: String,   // "FREE", "TEAM", "ENTERPRISE"
    };

    entity Project in [Workspace] {
        name: String,
        key: String,
    };

    entity Environment in [Workspace] {
        name: String,
        tier: String,   // "development", "staging", "production"
    };

    entity Instance in [Environment] {
        name: String,
        engine: String, // "POSTGRES", "MYSQL", etc.
    };

    entity Database in [Project, Instance] {
        name: String,
        classification: String,
        labels: Set<String>,
    };

    entity Table in [Database] {
        name: String,
        has_pii: Bool,
    };

    entity Column in [Table] {
        name: String,
        semantic_type: String,
        classification: String,
    };

    // Actions
    action query appliesTo {
        principal: [User, ServiceAccount],
        resource: [Database, Table],
        context: {
            timestamp: String,
            hour: Long,
            day_of_week: String,
            sql_type: String,
            affected_rows: Long,
        }
    };

    action migrate appliesTo {
        principal: [User, ServiceAccount],
        resource: [Database],
        context: {
            migration_type: String,  // "DDL", "DML", "DATA"
            approval_status: String,
            approver_count: Long,
            has_backup: Bool,
        }
    };

    action export appliesTo {
        principal: [User, ServiceAccount],
        resource: [Database, Table],
        context: {
            format: String,  // "CSV", "JSON", "SQL"
            row_limit: Long,
        }
    };

    action manage appliesTo {
        principal: [User, ServiceAccount],
        resource: [Project, Instance, Database],
    };
}
```

### FR-003: Cedar Policy Templates

```cedar
// Template: Time-Based Access Control
permit(
    principal in Bytebase::Role::"projectDeveloper",
    action == Bytebase::Action::"query",
    resource
) when {
    context.hour >= 9 && context.hour <= 18
    && context.day_of_week != "Saturday"
    && context.day_of_week != "Sunday"
};

// Template: Production Migration Governance
forbid(
    principal,
    action == Bytebase::Action::"migrate",
    resource in Bytebase::Environment::"production"
) unless {
    context.approval_status == "APPROVED"
    && context.approver_count >= 2
};

// Template: PII Data Access
forbid(
    principal,
    action == Bytebase::Action::"query",
    resource
) when {
    resource.classification == "PII"
    && !(principal in Bytebase::Role::"workspaceAdmin")
    && !(principal in Bytebase::Role::"workspaceDBA")
} unless {
    context has "masking_applied" && context.masking_applied == true
};

// Template: Destructive Operation Prevention
forbid(
    principal,
    action == Bytebase::Action::"migrate",
    resource
) when {
    context.migration_type == "DDL"
    && context.sql_type in ["DROP", "TRUNCATE"]
    && !(principal in Bytebase::Role::"workspaceAdmin")
};

// Template: Export Rate Limiting
forbid(
    principal,
    action == Bytebase::Action::"export",
    resource
) when {
    context.row_limit > 10000
    && resource.classification == "SENSITIVE"
};
```

### FR-004: Cedar Policy Conflict Analysis

```go
// CedarAnalyzer provides static analysis for Cedar policies.
type CedarAnalyzer struct {
    policySet *cedar.PolicySet
    schema    *cedar.Schema
}

// DetectConflicts finds policies that may conflict.
func (a *CedarAnalyzer) DetectConflicts() ([]PolicyConflict, error)

// ValidateCoverage ensures all actions have at least one policy.
func (a *CedarAnalyzer) ValidateCoverage(actions []string) ([]string, error)

// SimulateRequest tests a hypothetical request against policies.
func (a *CedarAnalyzer) SimulateRequest(req *EvaluationRequest) (*SimulationResult, error)
```

### FR-005: Cedar Entity Data Loader

```go
type CedarDataLoader struct {
    store        *store.Store
    entityStore  *cedar.EntityStore
    syncInterval time.Duration
}

// SyncAll loads Bytebase entities into Cedar entity store.
func (l *CedarDataLoader) SyncAll(ctx context.Context) error {
    // Load users → User entities
    // Load groups → Group entities with membership
    // Load roles → Role entities
    // Load projects → Project entities
    // Load environments → Environment entities
    // Load instances → Instance entities with environment parent
    // Load databases → Database entities with project+instance parents
}
```

---

## 3. Yêu cầu kỹ thuật

| Component                          | File/Package                                         | Thay đổi                                    |
|------------------------------------|------------------------------------------------------|----------------------------------------------|
| CedarEngine                       | `backend/component/policy/cedar/engine.go`           | New: Cedar engine implementation             |
| CedarConfig                       | `backend/component/policy/cedar/config.go`           | New: Cedar engine configuration              |
| Cedar Schema                      | `backend/component/policy/cedar/schema.go`           | New: Bytebase Cedar schema definition        |
| Cedar Data Loader                 | `backend/component/policy/cedar/data_loader.go`      | New: Bytebase → Cedar entity sync            |
| Cedar Analyzer                    | `backend/component/policy/cedar/analyzer.go`         | New: conflict detection + coverage analysis  |
| Cedar Templates                   | `backend/component/policy/cedar/templates/`          | New: pre-built Cedar policy templates        |
| Plugin registration               | `backend/component/policy/cedar/init.go`             | New: engine factory registration             |

### 3.1 Go Dependencies

```go
require (
    github.com/cedar-policy/cedar-go v0.x.x    // Cedar Go bindings
)
```

---

## 4. Security Considerations

| Concern                          | Mitigation                                                    |
|----------------------------------|---------------------------------------------------------------|
| Cedar policy injection           | Schema validation prevents invalid entity types               |
| Entity data exposure             | Entity store scoped per workspace                             |
| Formal verification cost         | Conflict analysis runs async, cached results                  |
| Cedar language limitations       | Document unsupported use cases, recommend OPA/Rego for complex logic |

---

## 5. Test Cases

| Test ID | Mô tả                                                | Expected Result                           |
|---------|--------------------------------------------------------|-------------------------------------------|
| TC-001  | Cedar: load schema + simple permit policy             | Correct allow decision                     |
| TC-002  | Cedar: forbid policy with condition                   | Correct deny decision                      |
| TC-003  | Cedar: entity hierarchy (User in Group in Role)       | Hierarchical evaluation works              |
| TC-004  | Cedar: time-based access template                     | Denied outside hours, allowed within       |
| TC-005  | Cedar: production migration governance                | Requires 2 approvers                       |
| TC-006  | Cedar: conflict detection (permit vs forbid)          | Conflict identified and reported           |
| TC-007  | Cedar: entity data loader sync                        | All Bytebase entities available            |
| TC-008  | Cedar: invalid policy syntax                          | Validation error returned                  |
| TC-009  | Cedar: PII access with masking obligation             | Forbid unless masking applied              |

---

## 6. Rollout Plan

| Phase   | Mô tả                                         | Timeline       |
|---------|------------------------------------------------|----------------|
| Phase 1 | Cedar Go SDK integration + schema definition   | Sprint 1       |
| Phase 2 | Cedar engine implementation                    | Sprint 1-2     |
| Phase 3 | Entity data loader + sync                      | Sprint 2       |
| Phase 4 | Cedar policy templates                         | Sprint 3       |
| Phase 5 | Conflict analyzer + static analysis            | Sprint 3-4     |
| Phase 6 | Integration testing                            | Sprint 4       |
