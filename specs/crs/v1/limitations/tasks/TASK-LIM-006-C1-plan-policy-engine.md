# TASK-LIM-006-C1: Dynamic Plan Policy Engine

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-006 |
| Phase | C — Architecture Change |
| Priority | P1 |
| Depends On | TASK-LIM-006-A1, TASK-LIM-006-A2, TASK-LIM-006-B1 |
| Est. | L (~350 LoC) |

## Objective

Replace static `plan.yaml` reads with dynamic `PlanPolicyEngine` that supports DB-stored overrides. Self-hosted operators can customize feature gates without code rebuild.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/enterprise/policy_engine.go` |
| MODIFY | `backend/enterprise/license.go` — use PolicyEngine |
| CREATE | `backend/api/v1/subscription_service_ext.go` — ComparePlans, UpgradeHints |

## Specification

### `policy_engine.go`

```go
type PlanPolicyEngine struct {
    store           *store.Store
    defaultPolicies map[api.PlanType]*PlanPolicy  // from plan.yaml
}

type PlanPolicy struct {
    MaxInstances   int      `json:"maxInstances"`
    MaxSeats       int      `json:"maxSeats"`
    Features       []string `json:"features"`
    AuditRetention string   `json:"auditRetention"`
}
```

Key method:
```go
func (e *PlanPolicyEngine) GetPolicy(ctx, plan) *PlanPolicy {
    // Check DB override: setting key "plan.policy.{plan}"
    // If found: merge with defaults
    // If not: return compiled defaults from plan.yaml
}
```

### LicenseService refactor

Replace direct `plan.yaml` reads with `policyEngine.GetPolicy()`:
- `GetInstanceLimit()` → `policy.MaxInstances`
- `IsFeatureEnabled()` → check `policy.Features` list

### API endpoints

- `ComparePlans()` — returns all plans with limits, features, current usage
- `GetUpgradeHints()` — quota warnings, recently denied features

## Acceptance Criteria

- [ ] PolicyEngine loads defaults from plan.yaml
- [ ] DB override merges with defaults (override wins)
- [ ] LicenseService uses PolicyEngine (not direct yaml)
- [ ] No DB override → identical behavior to current
- [ ] ComparePlans returns accurate plan comparison
- [ ] UpgradeHints shows quota warnings at ≥80% usage
