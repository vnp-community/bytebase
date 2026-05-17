# Solution: Policy Enforcement Point Framework

| Field | Value |
|---|---|
| **SOL ID** | SOL-POL-006 |
| **CR Reference** | CR-POL-006 |
| **Status** | Proposed |
| **Created** | 2026-05-17 |
| **Dependencies** | SOL-POL-001 |

---

## 1. Architecture Mapping

| CR Component | Target Layer | Rationale |
|---|---|---|
| PEP Interface | **L3 — Security** | Enforcement points sit in interceptor/security chain |
| PEP Registry | **L5 — Component** | Shared registry, injected into services |
| API PEP | **L3 — Security** | Replaces inline ACL checks in interceptor chain |
| Database Query PEP | **L4 — Service** | Integrates into SQLService.Query() |
| Migration PEP | **L6 — Runner** | Integrates into TaskRun executor pipeline |
| Export PEP | **L5 — Component** | Integrates into Export component |
| External PEP | **L7 — Plugin** | HTTP-based plugin for Envoy/Kong sidecars |

---

## 2. Package Structure

```
backend/component/policy/pep/
├── pep.go             ← PEP interface + PEPType enums
├── registry.go        ← PEPRegistry: find + route enforcement requests
├── api.go             ← APIPep: ConnectRPC/REST interceptor enforcement
├── query.go           ← DatabaseQueryPEP: query access + masking obligations
├── migration.go       ← MigrationPEP: schema change governance
├── export.go          ← ExportPEP: export restrictions + row limits
├── webhook.go         ← WebhookPEP: notification policy compliance
├── external.go        ← ExternalPEP: HTTP callback to Envoy/Kong/custom
└── metrics.go         ← PEP-specific Prometheus metrics
```

---

## 3. Key Design Decisions

### 3.1 PEP Interface

```go
type PolicyEnforcementPoint interface {
    Name() string
    Type() PEPType

    // Enforce evaluates policy and applies enforcement action.
    Enforce(ctx context.Context, req *EnforcementRequest) (*EnforcementResult, error)

    // CanHandle checks if this PEP handles the given request type.
    CanHandle(req *EnforcementRequest) bool
}

type PEPType int
const (
    PEPTypeAPI       PEPType = iota  // API gateway level
    PEPTypeDatabase                   // Database query level
    PEPTypeMigration                  // Schema change level
    PEPTypeExport                     // Data export level
    PEPTypeWebhook                    // Notification level
    PEPTypeExternal                   // External sidecar
)

type EnforcementResult struct {
    Allowed      bool
    Decision     *PolicyDecision
    Obligations  []*AppliedObligation
    Metadata     map[string]string
}

type AppliedObligation struct {
    Type       string  // "mask", "audit", "rate_limit", "notify"
    Status     string  // "applied", "skipped", "failed"
    Details    map[string]string
}
```

### 3.2 PEP Registry — Routing Enforcement Requests

```go
type PEPRegistry struct {
    peps          map[string]PolicyEnforcementPoint
    policyManager *PolicyManager
    metrics       *PEPMetrics
}

func (r *PEPRegistry) Enforce(ctx, req *EnforcementRequest) (*EnforcementResult, error) {
    // 1. Find matching PEPs via CanHandle()
    // 2. Evaluate policy via PolicyManager
    // 3. Apply obligations (masking, audit, rate_limit)
    // 4. Return enforcement result
    // 5. Log to Bus.PolicyDecisionChan
}

func (r *PEPRegistry) RegisterPEP(pep PolicyEnforcementPoint) error
```

### 3.3 API PEP — ACL Interceptor Replacement

Current ACL interceptor (`acl.go`, 19KB) performs inline IAM checks. API PEP wraps this:

```go
type APIPep struct {
    policyManager *PolicyManager
    metrics       *PEPMetrics
}

func (p *APIPep) Enforce(ctx, req *EnforcementRequest) (*EnforcementResult, error) {
    evalReq := &EvaluationRequest{
        Resource: req.Resource,
        Subject:  req.Subject,
        Action:   req.Action,
        Context:  req.Context,
    }

    // Evaluate via PolicyManager (routes to CEL/OPA/Cedar as configured)
    decision, err := p.policyManager.Evaluate(ctx, evalReq)
    if err != nil {
        return nil, err
    }

    return &EnforcementResult{
        Allowed:  decision.Allowed,
        Decision: decision,
    }, nil
}
```

**Refactor path** in `acl.go`:

```go
// BEFORE:
allowed, err := iamManager.CheckPermission(ctx, permission, user, workspaceID)

// AFTER:
result, err := pepRegistry.Enforce(ctx, &EnforcementRequest{
    Type:     PEPTypeAPI,
    Resource: &PolicyResource{Type: resourceType, ID: resourceID},
    Subject:  &PolicySubject{Type: user.Type, ID: user.Email, Roles: roles},
    Action:   permission,
})
if !result.Allowed {
    return status.Errorf(codes.PermissionDenied, result.Decision.Reason)
}
```

### 3.4 Database Query PEP — Masking Obligations

Integrates into `SQLService.Query()` → replaces inline `MaskingEvaluator`:

```go
type DatabaseQueryPEP struct {
    policyManager *PolicyManager
    masker        *masker.Masker  // Existing masker component (L5)
}

func (p *DatabaseQueryPEP) Enforce(ctx, req) (*EnforcementResult, error) {
    decision, _ := p.policyManager.Evaluate(ctx, evalReq)

    result := &EnforcementResult{Allowed: decision.Allowed, Decision: decision}

    // Apply masking obligations
    for _, obligation := range decision.Obligations {
        if obligation.Type == "mask" {
            level := obligation.Parameters["level"]  // "NONE", "PARTIAL", "FULL"
            result.Obligations = append(result.Obligations, &AppliedObligation{
                Type:    "mask",
                Status:  "applied",
                Details: map[string]string{"level": level},
            })
        }
    }
    return result, nil
}
```

**Refactor path** in `masking_evaluator.go`:

```go
// BEFORE:
maskingLevel := evaluator.EvaluateMaskingLevel(column, user, accessGrants)

// AFTER:
result, _ := pepRegistry.Enforce(ctx, &EnforcementRequest{
    Type:     PEPTypeDatabase,
    Resource: &PolicyResource{Type: "column", ID: column.Name, Parent: dbResource},
    Subject:  currentSubject,
    Action:   "bb.databases.query",
})
maskingLevel := result.Obligations[0].Details["level"]
```

### 3.5 Migration PEP — Task Execution Gate

Integrates into `PendingScheduler` (L6 Runner):

```go
type MigrationPEP struct {
    policyManager *PolicyManager
}

func (p *MigrationPEP) Enforce(ctx, req) (*EnforcementResult, error) {
    // Evaluate migration governance policies
    // Check: time window, approval status, approver count, backup status
    decision, _ := p.policyManager.Evaluate(ctx, &EvaluationRequest{
        Resource: req.Resource,
        Subject:  req.Subject,
        Action:   "bb.databases.migrate",
        Context:  map[string]any{
            "migration_type":    req.Context["migration_type"],
            "approval_status":   req.Context["approval_status"],
            "approver_count":    req.Context["approver_count"],
            "sql_statement_type": req.Context["sql_type"],
        },
    })
    return &EnforcementResult{Allowed: decision.Allowed, Decision: decision}, nil
}
```

### 3.6 External PEP — Sidecar Integration

```go
type ExternalPEP struct {
    name     string
    endpoint string        // HTTP endpoint for policy check
    client   *http.Client
    timeout  time.Duration // Default: 500ms
}

func (p *ExternalPEP) Enforce(ctx, req) (*EnforcementResult, error) {
    // POST to external endpoint (Envoy, Kong OPA plugin, custom)
    // Body: enforcement request as JSON
    // Response: allow/deny + obligations
    resp, err := p.client.Post(p.endpoint, "application/json", body)
    return parseExternalResponse(resp)
}
```

---

## 4. Integration Points

### Existing Code Refactoring

| Current Code | Refactored To | PEP Type |
|---|---|---|
| `acl.go` (19KB) — `isRequestAllowed()` | `APIPep.Enforce()` | API |
| `masking_evaluator.go` (12KB) | `DatabaseQueryPEP.Enforce()` | Database |
| `pending_scheduler.go` — environment policy | `MigrationPEP.Enforce()` | Migration |
| `export.go` — export restrictions | `ExportPEP.Enforce()` | Export |

### Bootstrap Integration

```
Server Bootstrap:
  5.5 policyManager = policy.NewManager(...)
  5.6 pepRegistry = pep.NewRegistry(policyManager)    ← NEW
      ├── Register APIPep
      ├── Register DatabaseQueryPEP
      ├── Register MigrationPEP
      ├── Register ExportPEP
      └── Register ExternalPEP (if configured)
```

---

## 5. Prometheus Metrics

```go
var (
    pepEnforceTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "pep_enforce_total"},
        []string{"pep_type", "decision"},
    )
    pepEnforceDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{Name: "pep_enforce_duration_seconds"},
        []string{"pep_type"},
    )
    pepObligationApplied = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "pep_obligation_applied_total"},
        []string{"pep_type", "obligation_type", "status"},
    )
)
```

---

## 6. Risk & Mitigation

| Risk | Mitigation |
|---|---|
| ACL refactor breaks auth | Shadow mode: run APIPep in parallel with existing ACL, compare results |
| Masking refactor breaks data protection | Unit test every masking level combination before switch |
| External PEP timeout | 500ms timeout + automatic bypass to local PEP evaluation |
| PEP chain ordering | Strict registry ordering: API → Database → Migration → Export |
