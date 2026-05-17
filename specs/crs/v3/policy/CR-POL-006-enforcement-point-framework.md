# Change Request: Policy Enforcement Point Framework

| Field | Value |
|---|---|
| **CR ID** | CR-POL-006 |
| **Title** | Policy Enforcement Point (PEP) Framework |
| **Plan** | ENTERPRISE |
| **Priority** | P1 — High |
| **Status** | Draft |
| **Created** | 2026-05-17 |
| **Author** | VNP AI Ops Team |
| **Dependencies** | CR-POL-001 |

---

## 1. Tổng quan

### 1.1 Mô tả
Thiết kế **Policy Enforcement Point (PEP) Framework** — hệ thống pluggable enforcement cho phép thực thi chính sách tại nhiều điểm khác nhau trong Bytebase pipeline: API gateway, database operations, schema changes, data access, CI/CD webhooks.

### 1.2 Bối cảnh
Hiện tại enforcement nằm rải rác:
- **ACL Interceptor** (`acl.go`, 19KB) — IAM permission checks
- **MaskingEvaluator** (`masking_evaluator.go`, 12KB) — inline masking evaluation
- **PendingScheduler** — environment policy checks cho task scheduling
- **PlanCheck** — SQL review rule evaluation

Không có framework thống nhất → thêm enforcement point mới requires modifying core code.

### 1.3 Mục tiêu
- Pluggable PEP interface cho nhiều enforcement points
- Pre-built PEPs cho: API access, database query, schema migration, data export
- External PEP support: Envoy, Kong, Traefik sidecars
- Obligation enforcement (masking, audit, rate limiting)
- PEP metrics và tracing

---

## 2. Yêu cầu chức năng

### FR-001: PEP Interface

```go
// PolicyEnforcementPoint evaluates and enforces policies at a specific point.
type PolicyEnforcementPoint interface {
    Name() string
    Type() PEPType  // API, DATABASE, MIGRATION, EXPORT, WEBHOOK

    // Enforce evaluates policy and applies enforcement action.
    Enforce(ctx context.Context, req *EnforcementRequest) (*EnforcementResult, error)

    // CanHandle checks if this PEP handles the given request type.
    CanHandle(req *EnforcementRequest) bool
}

type EnforcementResult struct {
    Allowed      bool
    Decision     *PolicyDecision
    Obligations  []*AppliedObligation  // Applied obligations (masking, audit, etc.)
    Metadata     map[string]string
}

type AppliedObligation struct {
    Type       string  // "mask", "audit", "rate_limit", "notify"
    Status     string  // "applied", "skipped", "failed"
    Details    map[string]string
}
```

### FR-002: Pre-built PEPs

| PEP | Enforcement Point | Policy Decisions |
|---|---|---|
| **APIPep** | ConnectRPC/REST interceptor | API access allow/deny |
| **DatabaseQueryPEP** | SQLService.Query() | Query access + masking obligations |
| **MigrationPEP** | TaskRun executor | Schema change governance |
| **DataExportPEP** | Export component | Export restrictions + row limits |
| **WebhookPEP** | Webhook manager | Notification policy compliance |

### FR-003: PEP Registry

```go
type PEPRegistry struct {
    peps          map[string]PolicyEnforcementPoint
    policyManager *PolicyManager
    metrics       *PEPMetrics
}

// Enforce finds the appropriate PEP and enforces.
func (r *PEPRegistry) Enforce(ctx context.Context, req *EnforcementRequest) (*EnforcementResult, error)

// RegisterPEP adds a new enforcement point.
func (r *PEPRegistry) RegisterPEP(pep PolicyEnforcementPoint) error
```

### FR-004: External PEP Integration

Support external enforcement via sidecar proxies:
- **Envoy** — OPA Envoy Plugin for API-level enforcement
- **Kong** — Kong OPA Plugin for gateway-level enforcement
- **Custom HTTP** — generic HTTP-based PEP for any proxy

```go
type ExternalPEP struct {
    name     string
    endpoint string       // HTTP endpoint for policy check
    client   *http.Client
    timeout  time.Duration
}
```

---

## 3. Yêu cầu kỹ thuật

| Component | File/Package | Thay đổi |
|---|---|---|
| PEP interface | `backend/component/policy/pep/pep.go` | New |
| PEP Registry | `backend/component/policy/pep/registry.go` | New |
| API PEP | `backend/component/policy/pep/api.go` | New |
| Database Query PEP | `backend/component/policy/pep/query.go` | New |
| Migration PEP | `backend/component/policy/pep/migration.go` | New |
| Export PEP | `backend/component/policy/pep/export.go` | New |
| External PEP | `backend/component/policy/pep/external.go` | New |
| ACL Interceptor | `backend/api/v1/acl.go` | Refactor: delegate to API PEP |
| MaskingEvaluator | `backend/api/v1/masking_evaluator.go` | Refactor: delegate to Query PEP |

---

## 4. Test Cases

| Test ID | Mô tả | Expected Result |
|---|---|---|
| TC-001 | API PEP: allow request | Request proceeds |
| TC-002 | API PEP: deny request | 403 Forbidden returned |
| TC-003 | Query PEP: apply masking obligation | Masking applied to results |
| TC-004 | Migration PEP: require approval | Task blocked until approved |
| TC-005 | Export PEP: enforce row limit | Export capped at limit |
| TC-006 | External PEP: HTTP callback | Decision from external service |
| TC-007 | PEP metrics | Counters per PEP type |

---

## 5. Rollout Plan

| Phase | Mô tả | Timeline |
|---|---|---|
| Phase 1 | PEP interface + registry | Sprint 1 |
| Phase 2 | API PEP + ACL refactor | Sprint 1-2 |
| Phase 3 | Query PEP + masking refactor | Sprint 2-3 |
| Phase 4 | Migration + Export PEPs | Sprint 3 |
| Phase 5 | External PEP support | Sprint 4 |
| Phase 6 | Integration testing | Sprint 4-5 |
